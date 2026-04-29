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
    // Definition returns the tool definition in OpenAI function calling format.
    Definition() ToolDefinition
}

// ToolDefinition describes a tool for the LLM.
type ToolDefinition struct {
    Type     string                 `json:"type"` // "function"
    Function FunctionDefinition     `json:"function"`
}

type FunctionDefinition struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"` // JSON Schema object
    Strict      bool                   `json:"strict,omitempty"`
}

// ToolExecutor executes a tool call.
type ToolExecutor interface {
    // Execute runs the tool with the given arguments.
    // Returns (result string, error).
    Execute(args json.RawMessage) (string, error)
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
func (r *Registry) List() []ToolDefinition  // For sending to LLM
func (r *Registry) Execute(name string, args json.RawMessage) (string, error)
```

## Streaming Tool Call Parser

During streaming, tool call arguments arrive as incremental JSON patches:

```
Chunk 1: {"name":"read","arguments":"{\"p"}
Chunk 2: {"name":"read","arguments":"ath\":\""}
Chunk 3: {"name":"read","arguments":"internal/"}
Chunk 4: {"name":"read","arguments":"tools/"}
Chunk 5: {"name":"read","arguments":"edit.\"}
Chunk 6: {"name":"read","arguments":"go\"}"}
```

The parser must:
1. Accumulate arguments per tool call index
2. Attempt to parse as JSON on each update (for real-time display)
3. Handle incomplete JSON gracefully (use partial-json parsing)
4. Validate against schema only when the call is complete

```go
package tools

// Parser handles streaming tool call parsing
type Parser struct {
    calls map[int]*AccumulatedCall
}

type AccumulatedCall struct {
    ID        string
    Name      string
    Args      string        // Accumulated JSON string
    Parsed    map[string]interface{} // Best-effort parsed JSON
}

func (p *Parser) Update(index int, id, name, argsDelta string) *AccumulatedCall
func (p *Parser) Finalize() map[int]*AccumulatedCall
func (p *Parser) Get(index int) *AccumulatedCall
```

### Partial JSON Parsing

For displaying partial tool calls during streaming, use a fallback parser:

```go
func parsePartialJSON(s string) map[string]interface{} {
    // Try standard JSON parse first
    var obj map[string]interface{}
    if err := json.Unmarshal([]byte(s), &obj); err == nil {
        return obj
    }
    // Fallback: try to complete the JSON
    // - Close unclosed strings
    // - Close unclosed objects/arrays
    // - Remove trailing commas
    completed := completeJSON(s)
    if err := json.Unmarshal([]byte(completed), &obj); err == nil {
        return obj
    }
    return nil
}

func completeJSON(s string) string {
    // Heuristic JSON completion:
    // 1. Close any open string literals
    // 2. Close any open { with }
    // 3. Close any open [ with ]
    // 4. Remove trailing commas before closing brackets
}
```

## JSON Schema Validation

Validate tool arguments against the tool's JSON Schema:

```go
package tools

// Validator validates arguments against a JSON Schema.
type Validator struct{}

// Validate checks if args conform to the schema.
// Returns nil if valid, or a descriptive error.
func (v *Validator) Validate(schema map[string]interface{}, args map[string]interface{}) error {
    // Check required fields
    // Check types
    // Check additionalProperties
}
```

Keep validation lightweight — we're not implementing full JSON Schema, just the subset used by tool definitions (type, required, additionalProperties, enum).

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

1. [ ] Implement `internal/tools/tool.go`: Tool interface, ToolDefinition
2. [ ] Implement `internal/tools/registry.go`: Tool registry
3. [ ] Implement `internal/tools/parser.go`: Streaming tool call parser
4. [ ] Implement `internal/tools/validator.go`: JSON Schema validation (subset)
5. [ ] Implement `internal/tools/path_utils.go`: Path resolution
6. [ ] Implement `internal/tools/read.go`: Read tool with chunked reading
7. [ ] Implement `internal/tools/write.go`: Write tool
8. [ ] Implement `internal/tools/edit.go`: Edit tool with diff
9. [ ] Implement `internal/tools/bash.go`: Bash tool with timeout
10. [ ] Wire tools into agent loop (tool call → registry lookup → execute)
11. [ ] Test: Agent calls read/write/edit/bash tools correctly