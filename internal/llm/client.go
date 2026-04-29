package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/user/mmok/internal/app"
)

// Client wraps an OpenAI-compatible API endpoint.
type Client struct {
	config *app.Config
	client *http.Client
}

// New creates a new LLM client.
func New(cfg *app.Config) *Client {
	return &Client{
		config: cfg,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// ChatRequest is the request body for chat completions.
type ChatRequest struct {
	Model       string      `json:"model"`
	Messages    []ChatMsg   `json:"messages"`
	Stream      bool        `json:"stream"`
	Temperature float32     `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
}

// ChatMsg is a single message in the chat request.
type ChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse is a single SSE chunk from the streaming API.
type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Done    bool     `json:"done,omitempty"`
}

// Choice holds a delta from the streaming response.
type Choice struct {
	Delta      Delta `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// Delta is the partial content update.
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// StreamHandler is called for each content chunk.
type StreamHandler func(content string) error

// Chat streams a chat completion. Returns the full response text.
func (c *Client) Chat(messages []ChatMsg, handler StreamHandler) (string, error) {
	req := ChatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Stream:      true,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	url := c.config.Endpoint
	if !strings.HasSuffix(url, "/chat/completions") {
		url = strings.TrimSuffix(url, "/") + "/chat/completions"
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	return c.parseStream(resp.Body, handler)
}

// parseStream reads SSE events and calls the handler for each content chunk.
func (c *Client) parseStream(body io.Reader, handler StreamHandler) (string, error) {
	var fullText strings.Builder
	scanner := bufio.NewScanner(body)

	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// SSE data lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Done signal
		if data == "[DONE]" {
			break
		}

		// Parse the JSON chunk
		var resp ChatResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue // skip malformed chunks
		}

		// Extract content
		for _, choice := range resp.Choices {
			if choice.Delta.Content != "" {
				if handler != nil {
					if err := handler(choice.Delta.Content); err != nil {
						return fullText.String(), err
					}
				}
				fullText.WriteString(choice.Delta.Content)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fullText.String(), fmt.Errorf("reading stream: %w", err)
	}

	return fullText.String(), nil
}
