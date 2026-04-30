# Phase 2B: Agent — Tool Calls, JSON Repair, Model Quirks

## Prerequisites
- Phase 2A complete (streaming chat, thinking tokens, TUI wired)

## Goals
- Tool call accumulation from SSE stream (map + slice for ordered lookup)
- JSON repair for malformed tool arguments
- Schema validation and type coercion
- Tool interface + registry stubs (full implementations in Phase 3)
- Agent loop extension: tool call → execute → retry
- Model quirk handling (Gemma single-chunk, etc.)
- TUI rendering for tool calls and results

## LLM Client Extensions

### Tool Call Types

```go
package llm

type APIToolCall struct {
    ID   string `json:"id"`
    Type string `json:"type"` // "function"
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"`
    } `json:"function"`
}

// PartialTC tracks an in-progress tool call during streaming
type PartialTC struct {
    Index   *int   // nil if backend omitted it
    ID      string
    Name    string
    RawArgs string // Accumulated raw JSON string — do NOT parse mid-stream
}

// ToolSpec is the wire format for a tool definition sent to the API.
// Lives in llm package to avoid import cycle with tools package.
type ToolSpec struct {
    Type     string       `json:"type"` // always "function"
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Parameters  map[string]any `json:"parameters"` // JSON Schema object
}
```

### StreamEvent Extension

Add tool call events to the existing `StreamEvent`:

```go
type StreamEvent struct {
    Type          string       // "text" | "thinking" | "tool_call" | "done" | "error"
    Text          string       // Text delta (content field)
    ThinkingDelta string       // Thinking delta (reasoning_content field)
    ToolCall      *PartialTC   // Tool call delta (phase 2B)
    Usage         *Usage
    Stop          string
    Err           error
}
```

### SSE Parser Extension

Parse `choices[0].delta.tool_calls[i]` → tool call delta:
- First chunk has `index`, `id`, `name`
- Subsequent chunks have `index` + incremental `arguments`
- Match by `index` first, fall back to `id` (Gemma may drop `index` after first chunk)

## Tool Call Accumulator

```go
package llm

// AccumulateToolCall merges a partial tool call delta into the accumulator.
// Uses a map for fast lookup by ID and a slice to preserve insertion order.
// Returns (isNew bool, *PartialTC).
func AccumulateToolCall(
    toolCallMap map[string]*PartialTC,
    toolCallOrder []*PartialTC,
    delta *PartialTC,
) (bool, *PartialTC)
```

**Accumulator rules:**

1. **Match by `index` first, fall back to `id`**: If a chunk's `tool_calls[i].index` is absent but `id` is present, match against an in-progress tool call by `id`. This handles backends that drop `index` after the first chunk.

2. **Accumulate raw strings only**: `PartialTC.RawArgs` is a raw string concatenation. Never try to parse it as JSON mid-stream — Gemma can produce invalid intermediate JSON. Parse only at `done` time.

3. **Fill in `id`/`name` late**: If a chunk provides `id` or `name` but the current block doesn't have them yet, fill them in. Some backends split these across chunks.

4. **Default missing `arguments` to `""`**: When a tool call has no parameters, the `arguments` field may be absent entirely. Treat this as empty string.

### Model Quirk Handling

Different model families behave differently through OpenAI-compatible endpoints:

**Qwen (2.5 / 3)** — baseline
- Streams `index`, `id`, `name` in first chunk, then incremental `arguments`
- Well-behaved, matches OpenAI spec

**Gemma 2+** — quirks
- May emit entire tool call in a single chunk (no incremental args)
- May omit `index` in subsequent chunks for the same tool call
- Can produce malformed mid-stream JSON (split mid-escape, mid-string)
- No-arg tools may omit the `arguments` field entirely

**gpt-oss** — expected to follow spec faithfully

**Model quirk flag**: Extending the `model_quirks` config from Phase 2A:
- `no_thinking` — suppress thinking rendering (from phase 2A)
- `single_chunk_tools` — skip incremental accumulation, treat each tool call chunk as complete (Gemma fallback)

**Auto-detection**: If the first tool call chunk has no `index` but has `id`, auto-enable `single_chunk_tools` fallback. Log the detection.

## JSON Repair at Completion

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

## Schema Validation at Completion

After repair, validate and coerce arguments against the tool's declared schema:

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

## Tool Interface + Registry (Stubs)

Full tool implementations are Phase 3. Phase 2B defines the interfaces and registry so the agent loop can execute tool calls.

```go
package tools

// ToolDefinition is the flat domain type for a tool's metadata.
// No Type field, no Function nesting, no Executor field.
type ToolDefinition struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// Tool is the executable interface.
type Tool interface {
    Definition() ToolDefinition
    Execute(args json.RawMessage) (string, error)
}

// Registry holds available tools and converts to wire format.
type Registry struct {
    tools map[string]Tool
}

func NewRegistry() *Registry
func (r *Registry) Add(tool Tool)
func (r *Registry) Get(name string) Tool
func (r *Registry) All() []Tool
func (r *Registry) ToSpecs() []llm.ToolSpec  // ToolDefinition → ToolSpec conversion
```

**Type layout across packages** (no import cycles):
- `tools.ToolDefinition` — flat domain type
- `tools.Tool` — interface: `Definition() ToolDefinition` + `Execute(json.RawMessage) (string, error)`
- `llm.ToolSpec` — nested wire type sent to API
- Conversion `ToolDefinition → ToolSpec` happens in the `agent` package (via `Registry.ToSpecs()`). Neither `llm` nor `tools` imports the other.

## Agent Loop Extension (Phase 2B)

### Extended Event Types

```go
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
```

### Agent Loop Flow (Phase 2B — with tool calls)

```
Run(userMessage):
    append user message to history
    emit EventTurnStart

    loop:
        messages = buildContext()  // system + history (with compaction check in phase 4)
        eventChan = client.Stream(context, messages, toolSpecs)
        emit EventMessageStart

        var assistantText   strings.Builder
        var thinkingText    strings.Builder
        var toolCallMap     map[string]*PartialTC  // keyed by ID
        var toolCallOrder   []*PartialTC           // insertion-ordered

        for event := range eventChan:
            switch event.Type:
            case "thinking":
                thinkingText.WriteString(event.ThinkingDelta)
                emit EventThinkingDelta{event.ThinkingDelta}
            case "text":
                assistantText.WriteString(event.Text)
                emit EventTextDelta{event.Text}
            case "tool_call":
                isNew, tc = AccumulateToolCall(toolCallMap, toolCallOrder, event.ToolCall)
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
                        args, err := ParseToolArgs(tc.RawArgs)
                        if err != nil:
                            args = json.RawMessage(`{}`)  // executor will return error result

                        // Validate and coerce against tool schema
                        tool := registry.Get(tc.Name)
                        if tool != nil:
                            args = ValidateAndCoerce(tool.Definition().Parameters, args)

                        // Execute — each tool unmarshals json.RawMessage into its own struct
                        result, err := tool.Execute(args)
                        isError := (err != nil)

                        // 3. Append each tool result immediately after in matching order
                        append llm.Message{Role:"tool", ToolCallID:tc.ID, Content:result} to history
                        emit EventToolResult{tc.ID, tc.Name, result, isError}

                    continue loop  // Loop back for next LLM call
                else:
                    // No tool calls — append text-only assistant message
                    append llm.Message{Role:"assistant", Content:assistantText.String()} to history
                    break

        emit EventTurnEnd
```

### ChatRequest Extension

Add tools to the request:

```go
type ChatRequest struct {
    Model       string
    Messages    []Message
    Tools       []ToolSpec   // phase 2B addition
    Temperature float32
    MaxTokens   int
}
```

### Agent Interface Extension

```go
type Agent struct {
    client   *llm.Client
    config   *app.Config
    tools    *tools.Registry    // phase 2B addition
    messages []llm.Message
    tracker  *ContextTracker
    cancel   context.CancelFunc
}

func NewAgent(client *llm.Client, config *app.Config, tools *tools.Registry) *Agent
```

### TUI Rendering for Tool Calls

1. **Tool call start**: Show tool name + "executing..." with a spinner or indicator
2. **Tool call end**: Show tool name + parsed args (collapsed) + result
3. **Tool error**: Show tool name + error in red
4. **Tool result**: Show result text (collapsed if long, expandable)

### Message Type Mapping (Extended)

Phase 2B adds tool call mapping:

```
llm.Message{Role:"assistant", ToolCalls:[...]}  →  types.Message (MsgAssistant) + nested tool calls
llm.Message{Role:"tool", ToolCallID:..., Content:...}  →  types.NewToolResult(...)
```

### Tasks

1. [ ] Implement `internal/llm/accumulator.go`: Tool call accumulator — map+slice pair for ordered, id/index matching, raw concat
2. [ ] Implement `internal/llm/json_repair.go`: JSON repair (control chars, invalid escapes, close unclosed)
3. [ ] Implement `internal/llm/stream.go` extension: Parse `tool_calls` from SSE delta, emit `tool_call` events
4. [ ] Implement `internal/tools/tool.go`: `Tool` interface + flat `ToolDefinition` + `Registry`
5. [ ] Implement `internal/tools/validator.go`: `ValidateAndCoerce` with type coercion
6. [ ] Extend `internal/agent/loop.go`: Tool call loop — ordered execution, assistant msg before tool results
7. [ ] Extend `internal/agent/events.go`: Add `EventToolCallStart`, `EventToolCallUpdate`, `EventToolCallEnd`, `EventToolResult`
8. [ ] Extend TUI: Render tool calls/results in message view
9. [ ] Test: Tool call with Qwen 3 (incremental args + thinking tokens)
10. [ ] Test: Gemma single-chunk tool calls (auto-detect quirk)
11. [ ] Test: No-arg tools (missing `arguments` field)
12. [ ] Test: Parallel tool calls — verify tool_result order matches tool_calls array order in history
13. [ ] Test: JSON repair — malformed args recovered by three-layer fallback
14. [ ] Test: Schema coercion — string "42" → number 42
