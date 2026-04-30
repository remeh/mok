# Phase 2A: Agent — Streaming, Thinking Tokens, TUI Wiring

## Goals
- Rewrite LLM client with context-aware abort support
- SSE parser emitting typed events in real-time (text + thinking)
- Agent loop: stream → emit events → TUI renders (text-only, no tools yet)
- System prompt optimized for local LLMs
- Token tracking and context estimation
- Wire agent into TUI app (replace echo with real streaming)

## Why Custom HTTP Client

- Real-time SSE parsing for streaming
- Full control over headers, retries, error handling, abort via context
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
    Temperature float32
    MaxTokens   int
}

// Message for the LLM API (wire format)
type Message struct {
    Role    string `json:"role"`    // "system" | "user" | "assistant" | "tool"
    Content string `json:"content"`
    // For assistant messages with tool calls (added in phase 2B):
    ToolCalls []APIToolCall `json:"tool_calls,omitempty"`
    // For tool messages (added in phase 2B):
    ToolCallID string `json:"tool_call_id,omitempty"`
    Name       string `json:"name,omitempty"`
}

// StreamEvent is emitted by the SSE parser
type StreamEvent struct {
    Type          string     // "text" | "thinking" | "done" | "error"
    Text          string     // Text delta (content field)
    ThinkingDelta string     // Thinking delta (reasoning_content field)
    Usage         *Usage     // Token usage (on done event)
    Stop          string     // Stop reason
    Err           error      // Stream error
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

// Client talks to OpenAI-compatible endpoints
type Client struct {
    BaseURL      string
    BearerToken  string        // optional, for endpoints requiring auth
    HTTPClient   *http.Client
}

func NewClient(baseURL string, bearerToken string) *Client

// Stream sends a chat completion and returns a channel of events.
// The channel is closed when streaming completes.
// Cancelling ctx aborts the in-flight request immediately.
func (c *Client) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
```

### SSE Parser

The parser reads the HTTP response body line by line:

```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{"reasoning_content":"let me think..."}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

Key behaviors:
1. Parse each `data:` line as JSON (skip empty lines, `data: [DONE]`)
2. Extract `choices[0].delta.content` → text delta event
3. Extract `choices[0].delta.reasoning_content` → thinking delta event
4. Track `finish_reason`: "stop", "tool_calls", "length"
5. Parse final usage from last chunk
6. Emit all events to channel in real-time
7. Close channel on completion or error

### Thinking Token Handling

Both Qwen 3.x and Gemma 4 surface reasoning traces via a `reasoning_content` field in the SSE delta — llama-server translates each model's native thinking format into this unified field.

Example delta:
```
{"choices":[{"delta":{"reasoning_content":"let me think..."}}]}
{"choices":[{"delta":{"reasoning_content":null,"content":"The answer is 4"}}]}
```

Rules:
- **`reasoning_content` present and non-empty** → emit `EventThinkingDelta`, accumulate into `thinkingText` builder. Never append to LLM history.
- **`content` present and non-empty** → emit `EventTextDelta`, accumulate into `assistantText` builder. Append to history.
- The two fields are mutually exclusive within a single chunk in practice, but handle both being non-nil defensively.
- When `reasoning_content` transitions to `null` and `content` starts, thinking is complete.

**Model quirk flag**: Add an optional `model_quirks` field to `app.Config` (YAML: `model_quirks: ["no_thinking"]`) so per-model workarounds can be toggled without code changes. Currently anticipated values:
- `no_thinking` — suppress `reasoning_content` rendering even if model emits it (for models that produce spurious thinking output)

### Token Estimation

For local LLMs we don't always have tiktoken available. Use a pragmatic approach:

```go
// EstimateTokens estimates token count from text.
// Uses a simple heuristic: ~4 chars per token for English/text.
// Good enough for compaction thresholds, not for accurate reporting.
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
    messages []llm.Message
    tracker  *ContextTracker
    cancel   context.CancelFunc  // for abort support
}

func NewAgent(client *llm.Client, config *app.Config) *Agent

// Run starts the agent loop for a single user message.
// Streams the response and emits events in real-time.
// Phase 2A: text-only (no tool calls). Phase 2B adds tool call loop.
func (a *Agent) Run(ctx context.Context, userMessage string, events chan<- Event) error

// Abort stops the current streaming request (thread-safe, idempotent)
func (a *Agent) Abort()

// Messages returns the conversation history
func (a *Agent) Messages() []llm.Message

// AddMessage appends a message to history
func (a *Agent) AddMessage(msg llm.Message)
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

type EventThinkingDelta struct {
    Text string
}

type EventMessageEnd struct {
    Usage *llm.Usage
}

type EventTurnEnd struct{}

type EventError struct {
    Err error
}

// Phase 2B events (declared here, implemented in 2B):
// type EventToolCallStart struct { ... }
// type EventToolCallUpdate struct { ... }
// type EventToolCallEnd struct { ... }
// type EventToolResult struct { ... }
// type EventCompactionStart struct{}
// type EventCompactionEnd struct { TokensBefore int; TokensAfter int }
```

### Agent Loop Flow (Phase 2A — text only)

```
Run(userMessage):
    append user message to history
    emit EventTurnStart

    messages = buildContext()  // system + history
    eventChan = client.Stream(context, messages)
    emit EventMessageStart

    var assistantText  strings.Builder
    var thinkingText   strings.Builder

    for event := range eventChan:
        switch event.Type:
        case "thinking":
            thinkingText.WriteString(event.ThinkingDelta)
            emit EventThinkingDelta{event.ThinkingDelta}
        case "text":
            assistantText.WriteString(event.Text)
            emit EventTextDelta{event.Text}
        case "done":
            emit EventMessageEnd{event.Usage}
            // Strip thinking from content before saving to history
            append llm.Message{Role:"assistant", Content:assistantText.String()} to history
        case "error":
            emit EventError{event.Err}

    emit EventTurnEnd
```

### Abort Mechanism

```go
func (a *Agent) Run(ctx context.Context, userMessage string, events chan<- Event) error {
    ctx, cancel := context.WithCancel(ctx)
    a.cancel = cancel  // stored for Abort()
    defer cancel()
    // ... stream logic uses ctx ...
}

func (a *Agent) Abort() {
    if a.cancel != nil {
        a.cancel()  // cancels in-flight HTTP request via http.NewRequestWithContext
    }
}
```

TUI propagation: Ctrl-C in input → `AppModel.Update` → calls `Agent.Abort()`.

### System Prompt Builder

```go
package agent

func BuildSystemPrompt(config *PromptConfig) string {
    return `You are an expert coding assistant. You help users by reading files,
executing commands, editing code, and writing new files.

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

### TUI Wiring

Connect the agent to the TUI app:

1. `AppModel` holds an `*agent.Agent` and a `chan agent.Event`
2. On message submit:
   - Disable input area
   - Create a new assistant message placeholder in `MessageView`
   - Call `agent.Run(ctx, userInput, eventChan)` in a goroutine
3. Event handling in `AppModel.Update`:
   - `EventTurnStart` → set status bar to "thinking"
   - `EventMessageStart` → create streaming message in view
   - `EventTextDelta` → append text to streaming message
   - `EventThinkingDelta` → append to collapsed thinking block
   - `EventMessageEnd` → finalize message, show token usage
   - `EventTurnEnd` → re-enable input, show model name + tokens
   - `EventError` → show error in status bar, re-enable input
4. Thinking blocks render collapsed by default, expandable with a toggle

### Message Type Mapping

Conversion between wire format and UI display format:

```
llm.Message (wire)  →  types.Message (UI display)

llm.Message{Role:"assistant", Content:"..."}  →  types.NewMessage(MsgAssistant, "...")
llm.Message{Role:"user", Content:"..."}       →  types.NewMessage(MsgUser, "...")
```

Phase 2B will add tool call mapping:
```
llm.Message{Role:"assistant", ToolCalls:[...]}  →  types.Message + nested tool calls/results
```

### Tasks

1. [x] Rewrite `internal/llm/client.go`: HTTP client with `context.Context`, abort support, bearer token
2. [x] Implement `internal/llm/stream.go`: SSE parser → channel of `StreamEvent` (text + thinking + done + error)
3. [x] Implement `internal/llm/tokenizer.go`: Token estimation (`EstimateTokens`, `ContextTracker`)
4. [x] Implement `internal/agent/events.go`: Event types (`EventTurnStart`, `EventMessageStart`, `EventTextDelta`, `EventThinkingDelta`, `EventMessageEnd`, `EventTurnEnd`, `EventError`)
5. [x] Implement `internal/agent/prompt.go`: System prompt builder
6. [x] Implement `internal/agent/context.go`: Context window tracking (merged into `tokenizer.go`)
7. [x] Implement `internal/agent/loop.go`: Agent loop (text-only streaming)
8. [x] Implement `internal/agent/agent.go`: Agent facade with `Abort()`
9. [x] Wire agent into TUI app: `AppModel` holds `*agent.Agent`, events drive UI updates
10. [x] Render thinking blocks collapsed in TUI message view
11. [x] Test: Send a message, see real streaming response (not echo) — `./mmok -p "Say hello" -model gemma4-e4b` → 74 chars in 609ms
12. [x] Test: Thinking tokens filtered, don't appear in output — `qwen3.5-9b-thinking` → clean output, no `</think>` leakage
13. [x] Test: Abort kills in-flight request — `-t 6` on long prompt → partial output (823 chars) then clean cutoff
14. [x] Test: Token tracking — local estimate shown when API omits usage (llama-server quirk: no streaming usage)
