# Phase 2: Agent — Prompts, Streaming, OpenAI Endpoint

## Goals
- OpenAI-compatible streaming client (custom HTTP, not SDK)
- SSE parser that emits events in real-time
- Agent loop: stream → detect tool calls → execute → repeat
- System prompt optimized for local LLMs
- Token tracking and context estimation

## Why Custom HTTP Client

- Real-time SSE parsing for streaming tool call arguments
- Full control over headers, retries, error handling
- No OpenAI SDK type system baggage
- llama-server compatibility (may differ from OpenAI in edge cases)

## LLM Client

### Interface

```go
package llm

// ChatRequest is a single chat completion request.
type ChatRequest struct {
    Model       string
    Messages    []Message
    Tools       []ToolDefinition  // OpenAI function calling format
    Temperature float32
    MaxTokens   int
}

// Message for the LLM API
type Message struct {
    Role    string `json:"role"`    // "system" | "user" | "assistant" | "tool"
    Content string `json:"content"`
    // For assistant messages with tool calls:
    ToolCalls []APIToolCall `json:"tool_calls,omitempty"`
    // For tool messages:
    ToolCallID string `json:"tool_call_id,omitempty"`
    Name       string `json:"name,omitempty"`
}

type APIToolCall struct {
    ID   string `json:"id"`
    Type string `json:"type"` // "function"
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"`
    } `json:"function"`
}

// StreamEvent is emitted by the SSE parser
type StreamEvent struct {
    Type     string       // "text" | "tool_call" | "done" | "error"
    Text     string       // Text delta
    ToolCall *PartialTC   // Partial tool call during streaming
    Usage    *Usage       // Token usage (on done event)
    Stop     string       // Stop reason
    Err      error        // Stream error
}

type PartialTC struct {
    Index int
    ID    string
    Name  string
    Args  string // Accumulated JSON arguments (may be partial)
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

// Client talks to OpenAI-compatible endpoints
type Client struct {
    BaseURL   string
    APIKey    string
    HTTPClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client

// Stream sends a chat completion and returns a channel of events.
// The channel is closed when streaming completes.
func (c *Client) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
```

### SSE Parser

The parser reads the HTTP response body line by line:

```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","function":{"name":"read","arguments":"{\"p"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ath\":\""}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
```

Key behaviors:
1. Parse each `data:` line as JSON (skip empty lines, `data: [DONE]`)
2. Extract `choices[0].delta.content` → text delta event
3. Extract `choices[0].delta.tool_calls[i]` → tool call delta
   - Arguments arrive as incremental JSON patches, accumulate per index
   - First chunk has `id` and `name`, subsequent chunks have `arguments` only
4. Track `finish_reason`: "stop", "tool_calls", "length"
5. Parse final usage from last chunk
6. Emit all events to channel in real-time
7. Close channel on completion or error

### Token Estimation

For local LLMs we don't always have tiktoken available. Use a pragmatic approach:

```go
// EstimateTokens estimates token count from text.
// Uses a simple heuristic: ~4 chars per token for English/text.
// For more accuracy, integrate a Go tiktoken port if available.
func EstimateTokens(text string) int {
    return len([]rune(text)) / 4
}

// ContextTracker tracks total context tokens
type ContextTracker struct {
    Messages []Message
}

func (t *ContextTracker) TotalTokens() int
func (t *ContextTracker) AddMessage(msg Message)
func (t *ContextTracker) RemoveMessages(count int)
```

## Agent Loop

### Agent Interface

```go
package agent

// Agent manages the conversation loop
type Agent struct {
    client   *llm.Client
    config   *app.Config
    tools    *tools.Registry
    messages []Message
    tracker  *ContextTracker
}

func NewAgent(client *llm.Client, config *app.Config, tools *tools.Registry) *Agent

// Run starts the agent loop for a single user message.
// It streams the response, executes tool calls, and repeats until
// the assistant responds without tool calls.
// Events are sent to the provided channel in real-time.
func (a *Agent) Run(ctx context.Context, userMessage string, events chan<- Event) error

// Abort stops the current streaming request
func (a *Agent) Abort()

// Messages returns the conversation history
func (a *Agent) Messages() []Message

// AddMessage appends a message to history
func (a *Agent) AddMessage(msg Message)
```

### Event Types

```go
// Event is emitted by the agent loop
type Event interface {
    eventType() string
}

type EventTurnStart struct{}

type EventMessageStart struct {
    MessageID string
}

type EventTextDelta struct {
    Text string
}

type EventMessageEnd struct {
    Usage *llm.Usage
}

type EventToolCallStart struct {
    ToolCallID string
    Name       string
    RawArgs    string
}

type EventToolCallUpdate struct {
    ToolCallID string
    RawArgs    string
}

type EventToolCallEnd struct {
    ToolCallID string
    Name       string
    Args       string
}

type EventToolResult struct {
    ToolCallID string
    Name       string
    Result     string
    IsError    bool
}

type EventTurnEnd struct{}

type EventError struct {
    Err error
}

type EventCompactionStart struct{}
type EventCompactionEnd struct {
    TokensBefore int
    TokensAfter  int
}
```

### Agent Loop Flow

```
Run(userMessage):
    append user message to history
    emit EventTurnStart

    loop:
        messages = buildContext()  // system + history (with compaction check)
        if context too large:
            compact(messages)
            messages = buildContext()

        eventChan = client.Stream(context, messages, tools)
        emit EventMessageStart

        var assistantText strings.Builder
        var toolCalls map[int]*PartialTC

        for event := range eventChan:
            switch event.Type:
            case "text":
                assistantText.WriteString(event.Text)
                emit EventTextDelta{event.Text}
            case "tool_call":
                tc = accumulateToolCall(toolCalls, event.ToolCall)
                emit EventToolCallUpdate{tc.ID, tc.Args}
            case "done":
                emit EventMessageEnd{event.Usage}
                if event.Stop == "tool_calls":
                    // Execute tool calls
                    results = executeToolCalls(toolCalls)
                    for result in results:
                        append tool_result to history
                        emit EventToolResult{result}
                    continue loop  // Loop back for next LLM call
                else:
                    // No tool calls, turn complete
                    append assistant message to history
                    break

        emit EventTurnEnd
```

### System Prompt Builder

```go
package agent

func BuildSystemPrompt(config *PromptConfig) string {
    // Optimized for local LLMs:
    // - Clear, explicit tool call format
    // - JSON schema for each tool
    // - Examples of correct usage
    // - No ambiguity

    return `You are an expert coding assistant. You help users by reading files,
executing commands, editing code, and writing new files.

Available tools:
{TOOL_DEFINITIONS}

Guidelines:
- Use bash for file operations like ls, rg, find
- Be concise in your responses
- Show file paths clearly when working with files
- When editing files, use the edit tool with exact oldText matches
- Read files in chunks when possible (use offset/limit)

Current date: {DATE}
Working directory: {CWD}`
}
```

### Tasks

1. [ ] Implement `internal/llm/client.go`: HTTP client with SSE streaming
2. [ ] Implement `internal/llm/stream.go`: SSE parser, event channel
3. [ ] Implement `internal/llm/tokenizer.go`: Token estimation
4. [ ] Implement `internal/agent/message.go`: Message types
5. [ ] Implement `internal/agent/events.go`: Event types
6. [ ] Implement `internal/agent/prompt.go`: System prompt builder
7. [ ] Implement `internal/agent/context.go`: Context window tracking
8. [ ] Implement `internal/agent/loop.go`: Agent loop (stream → tools → repeat)
9. [ ] Implement `internal/agent/agent.go`: Agent facade
10. [ ] Wire agent into TUI app (connect events to UI updates)
11. [ ] Test: Send a message, see streaming response, verify token tracking
