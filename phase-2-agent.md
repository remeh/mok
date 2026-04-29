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
    Tools       []ToolSpec  // use llm.ToolSpec, not tools.ToolDefinition (avoids import cycle)
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
    Index   *int   // nil if backend omitted it (some backends drop it after first chunk)
    ID      string
    Name    string
    RawArgs string // Accumulated raw JSON string — do NOT parse mid-stream
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

// ToolSpec is the wire format for a tool definition sent to the API.
// Lives in the llm package to avoid an import cycle with the tools package.
// tools.ToolDefinition → llm.ToolSpec conversion happens at call site.
type ToolSpec struct {
    Type     string       `json:"type"` // always "function"
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// Client talks to OpenAI-compatible endpoints
type Client struct {
    BaseURL    string
    HTTPClient *http.Client
}

func NewClient(baseURL string) *Client

// Stream sends a chat completion and returns a channel of events.
// The channel is closed when streaming completes.
// The HTTP request is created with http.NewRequestWithContext so that
// cancelling ctx aborts the in-flight request immediately.
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

### Tool Call Accumulation

Tool calls from different model families behave differently through OpenAI-compatible endpoints. The accumulator must handle these variations:

**Qwen (2.5 / 3)** — baseline
- Streams `index`, `id`, `name` in first chunk, then incremental `arguments` in subsequent chunks
- Well-behaved, matches OpenAI spec
- Emits thinking via `reasoning_content` field in the delta (not `<think>` tags in `content`) — see "Thinking Token Handling" below

**Gemma 2+** — quirks
- May emit the entire tool call in a single chunk (no incremental args)
- May omit `index` in subsequent chunks for the same tool call
- Can produce malformed mid-stream JSON (split mid-escape, mid-string)
- No-arg tools may omit the `arguments` field entirely

**gpt-oss** — expected to follow spec faithfully

**Model quirk flag**: Add an optional `model_quirks` field to `app.Config` (YAML: `model_quirks: ["no_thinking"]`) so per-model workarounds can be toggled without code changes. Currently anticipated values:
- `no_thinking` — suppress `<think>` rendering even if model emits it (for models that produce spurious tags)
- `single_chunk_tools` — skip incremental accumulation, treat each tool call chunk as complete (Gemma fallback)

**Accumulator rules:**

1. **Match by `index` first, fall back to `id`**: If a chunk's `tool_calls[i].index` is absent but `id` is present, match against an in-progress tool call by `id`. This handles backends that drop `index` after the first chunk.

2. **Accumulate raw strings only**: `PartialTC.RawArgs` is a raw string concatenation. Never try to parse it as JSON mid-stream — Gemma can produce invalid intermediate JSON. Parse only at `done` time.

3. **Fill in `id`/`name` late**: If a chunk provides `id` or `name` but the current block doesn't have them yet, fill them in. Some backends split these across chunks.

4. **Default missing `arguments` to `""`**: When a tool call has no parameters, the `arguments` field may be absent entirely. Treat this as empty string; the schema validation step below will resolve it to `{}`.

### Thinking Token Handling

Both Qwen 3.x and Gemma 4 surface reasoning traces via a `reasoning_content` field in the SSE delta — llama-server translates each model's native thinking format (Qwen's `<think>` tags, Gemma's internal markers) into this unified field. No tag parsing is needed.

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

```go
// Add to StreamEvent:
type StreamEvent struct {
    Type          string     // "text" | "thinking" | "tool_call" | "done" | "error"
    Text          string     // Text delta (content field)
    ThinkingDelta string     // Thinking delta (reasoning_content field)
    ToolCall      *PartialTC
    Usage         *Usage
    Stop          string
    Err           error
}

// Add to agent events:
type EventThinkingDelta struct {
    Text string
}
```

### JSON Repair at Completion

When the stream ends and tool call arguments are complete, parse through a three-layer fallback:

```go
package llm

// ParseToolArgs parses accumulated raw JSON arguments with repair.
// Returns the parsed object, or nil if all parsing strategies fail.
// On failure, the raw string is still available for error reporting.
func ParseToolArgs(raw string) (json.RawMessage, error) {
    // Layer 1: try direct parse
    if json.Valid([]byte(raw)) {
        return json.RawMessage(raw), nil
    }
    // Layer 2: repair common malformations, then validate
    repaired := RepairJSON(raw)
    if json.Valid([]byte(repaired)) {
        return json.RawMessage(repaired), nil
    }
    // Layer 3: close unclosed strings/braces, then validate
    closed := CloseJSON(raw)
    if json.Valid([]byte(closed)) {
        return json.RawMessage(closed), nil
    }
    return nil, fmt.Errorf("failed to parse tool arguments: %s", raw)
}

// RepairJSON fixes common JSON malformations from LLM output:
// - Escapes raw control characters inside strings
// - Doubles backslashes before invalid escape characters
func RepairJSON(raw string) string

// CloseJSON closes unclosed strings, arrays, and objects:
//   {"key": "val  →  {"key": "val"}
//   ["a", "b"     →  ["a", "b"]
func CloseJSON(raw string) string
```

### Schema Validation at Completion

After repair, validate and coerce arguments against the tool's declared schema. This catches model hallucinations (extra fields, wrong types, missing required keys).

```go
package tools

// ValidateAndCoerce validates args against schema with type coercion.
// Returns corrected args on success, or the original args on failure (never blocks the turn).
// Validation failures are logged as warnings — the tool executor will return an error
// result that the model can retry.
func ValidateAndCoerce(schema map[string]any, args json.RawMessage) json.RawMessage
```

Coercion rules (match what the model likely intended):
- String `"42"` → number `42` when schema says `number`/`integer`
- String `"true"` → boolean `true` when schema says `boolean`
- Missing optional fields → omit (don't inject defaults)

**Type layout across packages** (no import cycles):
- `tools.ToolDefinition` — flat domain type: `{Name, Description, Parameters map[string]any}`. No `Type` field, no `Function` nesting, no `Executor` field.
- `tools.Tool` — interface: `Definition() ToolDefinition` + `Execute(json.RawMessage) (string, error)`
- `llm.ToolSpec` — nested wire type sent to API: `{Type:"function", Function:{Name, Description, Parameters}}`
- Conversion `ToolDefinition → ToolSpec` happens in the `agent` package. Neither `llm` nor `tools` imports the other.

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

        var assistantText  strings.Builder
        var thinkingText   strings.Builder
        // Use both a map (fast lookup by ID) and a slice (preserve insertion order).
        // Insertion order matters: tool_result messages must follow the assistant's
        // tool_calls array in exactly the same sequence, or some backends reject the request.
        var toolCallMap   map[string]*PartialTC  // keyed by ID
        var toolCallOrder []*PartialTC           // insertion-ordered

        for event := range eventChan:
            switch event.Type:
            case "thinking":
                // Qwen 3 <think> block — display only, never appended to history
                thinkingText.WriteString(event.ThinkingDelta)
                emit EventThinkingDelta{event.ThinkingDelta}
            case "text":
                assistantText.WriteString(event.Text)
                emit EventTextDelta{event.Text}
            case "tool_call":
                isNew, tc = accumulateToolCall(toolCallMap, toolCallOrder, event.ToolCall)
                    // match by index if present, else by id
                    // concatenate raw args string — do NOT parse
                    // fill in id/name if late
                if isNew:
                    toolCallOrder = append(toolCallOrder, tc)
                    emit EventToolCallStart{tc.ID, tc.Name}
                else:
                    emit EventToolCallUpdate{tc.ID, tc.RawArgs}
            case "done":
                emit EventMessageEnd{event.Usage}
                if event.Stop == "tool_calls":
                    // 1. Append the assistant turn to history first, with the full
                    //    tool_calls array in insertion order. The tool_result messages
                    //    that follow must reference the same IDs in the same sequence.
                    assistantMsg = llm.Message{
                        Role:      "assistant",
                        Content:   assistantText.String(),  // may be empty
                        ToolCalls: toAPIToolCalls(toolCallOrder),
                    }
                    append assistantMsg to history

                    // 2. Parse + validate + execute each tool call in order
                    for _, tc := range toolCallOrder:
                        emit EventToolCallEnd{tc.ID, tc.Name, tc.RawArgs}

                        // Parse accumulated args (three-layer repair) → json.RawMessage
                        args, err := llm.ParseToolArgs(tc.RawArgs)
                        if err != nil:
                            args = json.RawMessage(`{}`)  // executor will return an error result

                        // Validate and coerce against tool schema
                        tool := registry.Get(tc.Name)
                        if tool != nil:
                            args = tools.ValidateAndCoerce(tool.Definition().Parameters, args)

                        // Execute — each tool unmarshals json.RawMessage into its own struct
                        result, err := tool.Execute(args)
                        isError := (err != nil)
                        // 3. Append each tool result immediately after in matching order
                        append llm.Message{Role:"tool", ToolCallID:tc.ID, Content:result} to history
                        emit EventToolResult{tc.ID, tc.Name, result, isError}

                    continue loop  // Loop back for next LLM call
                else:
                    // No tool calls — strip thinking from content before saving to history
                    append llm.Message{Role:"assistant", Content:assistantText.String()} to history
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

1. [ ] Implement `internal/llm/client.go`: HTTP client with SSE streaming (`http.NewRequestWithContext` for abort support)
2. [ ] Implement `internal/llm/stream.go`: SSE parser, event channel, `<think>` tag detection and splitting
3. [ ] Implement `internal/llm/accumulator.go`: Tool call accumulator — map+slice pair for ordered, id/index matching, raw concat
4. [ ] Implement `internal/llm/json_repair.go`: JSON repair (control chars, invalid escapes, close unclosed)
5. [ ] Implement `internal/llm/tokenizer.go`: Token estimation
6. [ ] Implement `internal/tools/tool.go` stub: `Tool` interface + flat `ToolDefinition` (full implementations in phase 3)
7. [ ] Implement `internal/agent/message.go`: Message types
8. [ ] Implement `internal/agent/events.go`: Event types (including `EventThinkingDelta`)
9. [ ] Implement `internal/agent/prompt.go`: System prompt builder
10. [ ] Implement `internal/agent/context.go`: Context window tracking
11. [ ] Implement `internal/agent/loop.go`: Agent loop — ordered tool call execution, assistant msg before tool results, strip thinking from history
12. [ ] Implement `internal/agent/agent.go`: Agent facade
13. [ ] Wire agent into TUI app (connect events to UI updates, render thinking blocks collapsed)
14. [ ] Test: Send a message, see streaming response, verify token tracking
15. [ ] Test: Tool call with Qwen 3 (incremental args + thinking tokens), Gemma (single-chunk args), no-arg tools
16. [ ] Test: Parallel tool calls — verify tool_result order matches tool_calls array order in history
