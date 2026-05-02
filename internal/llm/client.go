package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to OpenAI-compatible endpoints.
type Client struct {
	BaseURL     string
	BearerToken string
	httpClient  *http.Client
	debug       DebugLogger
}

// NewClient creates a new LLM client.
func NewClient(baseURL, bearerToken string) *Client {
	return &Client{
		BaseURL:     baseURL,
		BearerToken: bearerToken,
		httpClient: &http.Client{
			Timeout: 1 * time.Hour,
		},
	}
}

// WithDebug sets the debug logger on the client.
func (c *Client) WithDebug(debug DebugLogger) *Client {
	c.debug = debug
	return c
}

// Message is the wire-format message for the LLM API.
type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	ToolCalls  []APIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	Name       string        `json:"name,omitempty"`
}

// APIToolCall is a tool call in the wire format.
type APIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ChatRequest is a single chat completion request.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolSpec
	Temperature float32
	MaxTokens   int
}

// ToolSpec is the wire format for a tool definition sent to the API.
type ToolSpec struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction is the function part of a ToolSpec.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// StreamEvent is emitted by the SSE parser.
type StreamEvent struct {
	Type          string
	Text          string
	ThinkingDelta string
	ToolCall      *PartialTC
	Usage         *Usage
	Stop          string
	Err           error
}

// PartialTC tracks an in-progress tool call during streaming.
type PartialTC struct {
	Index   *int
	ID      string
	Name    string
	RawArgs string
}

// Usage is token usage from the API.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Stream sends a chat completion and returns a channel of events.
// The channel is closed when streaming completes.
// Cancelling ctx aborts the in-flight request immediately.
func (c *Client) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	body, err := json.Marshal(c.buildRequestBody(req))
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := c.BaseURL
	if !strings.HasSuffix(url, "/chat/completions") {
		url = strings.TrimSuffix(url, "/") + "/chat/completions"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}

	if c.debug != nil {
		c.debug.Request("HTTP", "POST %s", url)
		c.debug.Request("HTTP", "Headers: Content-Type=application/json, Authorization=[REDACTED]")
		c.debug.JSON("HTTP", "Request body", c.buildRequestBody(req))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if c.debug != nil {
			c.debug.Response("HTTP", "Error response: %s, body: %s", resp.Status, string(errBody))
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	if c.debug != nil {
		c.debug.Response("HTTP", "Response: %s", resp.Status)
	}

	events := make(chan StreamEvent, 64)
	go c.parseStream(resp.Body, events)
	return events, nil
}

func (c *Client) buildRequestBody(req *ChatRequest) map[string]any {
	body := map[string]any{
		"model":        req.Model,
		"messages":     req.Messages,
		"stream":       true,
		"stream_options": map[string]any{"include_usage": true},
		"temperature":  req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	return body
}

func (c *Client) parseStream(body io.ReadCloser, events chan<- StreamEvent) {
	defer body.Close()
	defer close(events)

	scanner := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var finishReason string
	receivedDone := false
	emittedDone := false
	if c.debug != nil {
		c.debug.Response("SSE", "Stream parsing started")
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			receivedDone = true
			if c.debug != nil {
				c.debug.Response("SSE", "[DONE] received, ending stream")
			}
			break
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}

		// Parse usage from the top-level fields
		if usageRaw, ok := raw["usage"]; ok {
			if usageBytes, err := json.Marshal(usageRaw); err == nil {
				var usage Usage
				if err := json.Unmarshal(usageBytes, &usage); err == nil {
					events <- StreamEvent{
						Type:  "done",
						Usage: &usage,
						Stop:  finishReason,
					}
					if c.debug != nil {
						c.debug.Response("SSE", "Usage: prompt=%d completion=%d total=%d",
							usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
					}
				}
			}
			continue
		}

		choicesRaw, ok := raw["choices"]
		if !ok {
			continue
		}
		choicesArr, ok := choicesRaw.([]any)
		if !ok || len(choicesArr) == 0 {
			continue
		}

		for _, choiceAny := range choicesArr {
			choice, ok := choiceAny.(map[string]any)
			if !ok {
				continue
			}

			// Check finish_reason
			if fr, ok := choice["finish_reason"]; ok {
				if frStr, ok := fr.(string); ok && frStr != "" {
					finishReason = frStr
				}
			}

			delta, ok := choice["delta"].(map[string]any)
			if !ok {
				continue
			}

			// Handle reasoning_content (thinking tokens)
			if rc, ok := delta["reasoning_content"]; ok && rc != nil {
				if rcStr, ok := rc.(string); ok && rcStr != "" {
					events <- StreamEvent{
						Type:          "thinking",
						ThinkingDelta: rcStr,
					}
				}
			}

			// Handle content (regular text)
			if content, ok := delta["content"]; ok && content != nil {
				if contentStr, ok := content.(string); ok && contentStr != "" {
					events <- StreamEvent{
						Type: "text",
						Text: contentStr,
					}
				}
			}

			// Handle tool_calls (phase 2B)
			if tcRaw, ok := delta["tool_calls"]; ok && tcRaw != nil {
				tcs, ok := tcRaw.([]any)
				if !ok {
					continue
				}
				for _, tcAny := range tcs {
					tc, ok := tcAny.(map[string]any)
					if !ok {
						continue
					}
					partial := parsePartialTC(tc)
					if partial != nil {
						events <- StreamEvent{
							Type:     "tool_call",
							ToolCall: partial,
						}
						if c.debug != nil {
							c.debug.Event("SSE", "tool_call: index=%v id=%s name=%s args=%q",
								partial.Index, partial.ID, partial.Name, partial.RawArgs)
						}
					}
				}
			}
		}
	}

	// Log abnormal stream termination (no [DONE], no finish_reason)
	if !receivedDone && c.debug != nil {
		if scanErr := scanner.Err(); scanErr != nil {
			c.debug.Response("SSE", "STREAM ABORTED: scanner error: %v (finish_reason=%q)", scanErr, finishReason)
		} else if finishReason == "" {
			c.debug.Response("SSE", "STREAM ABORTED: EOF without [DONE] or finish_reason — likely client-side timeout (http.Client.Timeout)")
		} else {
			c.debug.Response("SSE", "STREAM WARNING: got finish_reason=%q but no [DONE] marker", finishReason)
		}
	}

	// Emit final done event if we haven't already (some backends omit usage chunk)
	if finishReason != "" && !emittedDone {
		events <- StreamEvent{
			Type: "done",
			Stop: finishReason,
		}
		if c.debug != nil {
			c.debug.Response("SSE", "Final done event: stop=%s", finishReason)
		}
	}
}

func parsePartialTC(raw map[string]any) *PartialTC {
	tc := &PartialTC{}

	if idx, ok := raw["index"]; ok {
		if idxNum, ok := idx.(float64); ok {
			i := int(idxNum)
			tc.Index = &i
		}
	}
	if id, ok := raw["id"]; ok {
		if idStr, ok := id.(string); ok {
			tc.ID = idStr
		}
	}
	if fn, ok := raw["function"]; ok {
		fnMap, ok := fn.(map[string]any)
		if !ok {
			return tc
		}
		if name, ok := fnMap["name"]; ok {
			if nameStr, ok := name.(string); ok {
				tc.Name = nameStr
			}
		}
		if args, ok := fnMap["arguments"]; ok {
			if argsStr, ok := args.(string); ok {
				tc.RawArgs = argsStr
			}
		}
	}
	if id, ok := raw["id"]; ok {
		if idStr, ok := id.(string); ok && tc.ID == "" {
			tc.ID = idStr
		}
	}

	return tc
}
