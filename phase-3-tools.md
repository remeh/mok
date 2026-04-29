# Phase 3: Tool Calling

## Goals
- Tool interface with JSON Schema definitions
- Streaming tool call parser (handles partial JSON during streaming)
- JSON Schema validation for tool arguments
- Built-in tools: read, write, edit, bash
- Tool registry for discovery and execution

## Tool Interface

```go
package tools

// Tool is a callable tool the agent can use.
type Tool interface {
    Definition() ToolDefinition
    Execute(args json.RawMessage) (string, error)
}

// ToolDefinition is the flat domain description of a tool.
// It is NOT the wire format sent to the API — the agent package converts it
// to llm.ToolSpec when building a ChatRequest.
type ToolDefinition struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"` // JSON Schema object
}
```

### JSON Schema for Tools

Each tool defines its parameters as JSON Schema (draft-07 subset):

```go
// Example: read tool schema
var readSchema = map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "path": map[string]interface{}{
            "type":        "string",
            "description": "Path to the file to read (relative or absolute)",
        },
        "offset": map[string]interface{}{
            "type":        "number",
            "description": "Line number to start reading from (1-indexed)",
        },
        "limit": map[string]interface{}{
            "type":        "number",
            "description": "Maximum number of lines to read",
        },
    },
    "required": []string{"path"},
    "additionalProperties": false,
}
```

## Tool Registry

```go
// Registry manages available tools.
type Registry struct {
    tools map[string]Tool
}

func NewRegistry() *Registry
func (r *Registry) Register(tool Tool)
func (r *Registry) Get(name string) (Tool, bool)
// List returns definitions for all registered tools.
// The agent package converts these to []llm.ToolSpec before sending to the API.
func (r *Registry) List() []ToolDefinition
func (r *Registry) Execute(name string, args json.RawMessage) (string, error)
```

## Streaming Tool Call Accumulation

Accumulation of incremental tool call arguments during streaming is handled entirely by `internal/llm/accumulator.go` (see phase 2). The `tools` package has no streaming parser — it only receives finalized `json.RawMessage` args when `Execute` is called.

## JSON Schema Validation

After the llm package repairs raw args into valid JSON, the tools package validates and coerces them:

```go
package tools

// ValidateAndCoerce validates args against the tool's JSON Schema with type coercion.
// Returns corrected args on success, or the original args if validation fails (never blocks the turn).
// Coercion: string "42" → int 42, string "true" → bool true when schema demands it.
// Validation failures are logged as warnings — the tool's Execute will return an error
// result that the model can retry.
func ValidateAndCoerce(schema map[string]any, args json.RawMessage) json.RawMessage
```

Keep validation lightweight — implement only the subset used by tool definitions: `type`, `required`, `additionalProperties`, `enum`.

## Built-in Tools

### read

Read file contents with offset/limit support:

```go
type ReadTool struct {
    CWD string
}

func (t *ReadTool) Definition() ToolDefinition { ... }
func (t *ReadTool) Execute(args json.RawMessage) (string, error) {
    // Parse args: path, offset (optional), limit (optional)
    // Resolve path relative to CWD
    // Read file
    // Apply offset/limit
    // Truncate very large outputs (max ~50KB or 2000 lines)
    // For images: detect mime type, return base64 or description
    // Return formatted content
}
```

Key behaviors:
- `offset` is 1-indexed line number
- `limit` is max lines to read
- Truncation: if output exceeds limits, show first N lines with "..." and continuation hint
- Image support: detect jpg/png/gif/webp, return description or base64

### write

Write content to a file:

```go
type WriteTool struct {
    CWD string
}

func (t *WriteTool) Execute(args json.RawMessage) (string, error) {
    // Parse args: path, content
    // Resolve path relative to CWD
    // Create parent directories if needed
    // Write content
    // Return success message with file size
}
```

### edit

Search/replace edits with diff output:

```go
type EditTool struct {
    CWD string
}

// EditArgs represents the edit tool arguments
type EditArgs struct {
    Path  string    `json:"path"`
    Edits []EditOp  `json:"edits"`
}

type EditOp struct {
    OldText string `json:"oldText"` // Exact text to find
    NewText string `json:"newText"` // Replacement text
}

func (t *EditTool) Execute(args json.RawMessage) (string, error) {
    // Parse args
    // Read file
    // For each edit: find OldText, replace with NewText
    // All edits match against ORIGINAL content (not incremental)
    // Generate unified diff
    // Write file
    // Return diff + line numbers of changes
}
```

Key behaviors:
- OldText must match exactly (including whitespace)
- Multiple edits in one call, all against original content
- No overlapping edits allowed
- Preserve line endings and BOM
- Return unified diff for review

### bash

Execute shell commands:

```go
type BashTool struct {
    CWD         string
    Timeout     time.Duration
}

func (t *BashTool) Execute(args json.RawMessage) (string, error) {
    // Parse args: command
    // Execute in shell (bash -c)
    // Capture stdout + stderr
    // Enforce timeout
    // Truncate output if too large
    // Return output
}
```

Key behaviors:
- Runs in a subshell
- Timeout: default 30 seconds
- Truncate output: max ~2000 lines or 50KB
- Capture exit code
- Working directory: CWD

## Path Utilities

```go
package tools

// ResolvePath resolves a path relative to CWD.
// Handles ~, relative paths, and absolute paths.
func ResolvePath(path, cwd string) (string, error)

// IsSafePath checks if a resolved path is within allowed directories.
func IsSafePath(resolved, cwd string) bool
```

## Tasks

1. [ ] Implement `internal/tools/tool.go`: `Tool` interface, flat `ToolDefinition`
2. [ ] Implement `internal/tools/registry.go`: Tool registry (`Register`, `Get`, `List`, `Execute`)
3. [ ] Implement `internal/tools/validate.go`: `ValidateAndCoerce` function (type/required/additionalProperties/enum subset)
4. [ ] Implement `internal/tools/path_utils.go`: Path resolution and safety check
5. [ ] Implement `internal/tools/read.go`: Read tool with offset/limit/truncation
6. [ ] Implement `internal/tools/write.go`: Write tool
7. [ ] Implement `internal/tools/edit.go`: Edit tool with unified diff output
8. [ ] Implement `internal/tools/bash.go`: Bash tool with timeout and output truncation
9. [ ] Wire tools into agent loop: `registry.List()` → convert to `[]llm.ToolSpec` in agent package
10. [ ] Test: Agent calls read/write/edit/bash tools correctly
11. [ ] Test: `ValidateAndCoerce` handles missing required field (warn + pass through), wrong type (coerce)