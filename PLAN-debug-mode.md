# Plan: Debug Mode Implementation

## Overview

Add a `-debug` flag to `mmok` that enables verbose logging of all internal agent operations, including:
- All LLM requests/responses (full message payloads)
- Agent loop state transitions
- Tool call execution details
- Event flow through the system
- Token tracking and context management

This will be essential for debugging tool calling implementation issues and understanding the agent loop behavior.

## Current State Analysis

The codebase already has good event infrastructure:
- `internal/agent/events.go` defines all agent events (turn_start, message_start, text_delta, thinking_delta, tool_call_start, tool_call_end, tool_result, etc.)
- `internal/llm/client.go` handles streaming from the LLM API
- `internal/agent/loop.go` contains the main agent loop logic
- Events flow from agent → TUI or prompt mode handler

What's missing:
- No centralized logging/debug output mechanism
- No visibility into HTTP request/response details
- No logging of tool execution internals
- No context/building/debugging of messages sent to LLM

## Implementation Phases

### Phase 1: Debug Logger Infrastructure

**Goal**: Create a centralized debug logging system that can be enabled/disabled.

**Files to create/modify**:
- `internal/agent/debug.go` (NEW) - Debug logger with categories and formatting

**Tasks**:
1. Create `DebugLogger` struct with:
   - Enabled flag
   - Category filters (optional, for future expansion)
   - Output writer (default: stderr)
   - Prefix formatting (timestamp, category, level)

2. Implement logging methods:
   - `Debug(category, format, args...)` - verbose debug info
   - `Info(category, format, args...)` - informational messages
   - `Request(category, format, args...)` - HTTP/API requests
   - `Response(category, format, args...)` - HTTP/API responses
   - `Event(category, format, args...)` - agent events
   - `Tool(category, format, args...)` - tool execution

3. Implement helper methods:
   - `JSON(category, string, v any)` - pretty-print JSON
   - `Dump(category, string, data []byte)` - dump raw bytes
   - `Separator(category)` - visual separator for log sections

4. Export a global debug logger instance for easy access

**Example output**:
```
[DEBUG] [2026-04-30 14:23:45] [AGENT] Turn started
[DEBUG] [2026-04-30 14:23:45] [CONTEXT] Building context with 3 messages
[DEBUG] [2026-04-30 14:23:45] [REQUEST] Sending to LLM:
{
  "model": "gemma4-e4b",
  "messages": [...],
  "tools": [...]
}
[DEBUG] [2026-04-30 14:23:46] [RESPONSE] Received SSE stream
[DEBUG] [2026-04-30 14:23:46] [EVENT] message_start
[DEBUG] [2026-04-30 14:23:46] [EVENT] thinking_delta: "Okay, let me think about this..."
[DEBUG] [2026-04-30 14:23:47] [EVENT] tool_call_start: id=call_123 name=read
[DEBUG] [2026-04-30 14:23:48] [TOOL] Executing read with args: {"path": "file.txt"}
[DEBUG] [2026-04-30 14:23:48] [TOOL] read result (234 bytes): "content..."
[DEBUG] [2026-04-30 14:23:48] [EVENT] tool_result: id=call_123 success=true
```

### Phase 2: Integrate Debug Logger into Agent Loop

**Goal**: Add debug logging throughout the agent loop.

**Files to modify**:
- `internal/agent/loop.go`
- `internal/agent/agent.go`

**Tasks**:
1. Add `debug *DebugLogger` field to `Agent` struct
2. Initialize debug logger in `NewAgent()`
3. Add debug logging at key points in `runLoop()`:
   - Turn start/end
   - Context building (log message count, token count)
   - Request building (log full request JSON)
   - Stream start/end
   - Each event received from stream (thinking, text, tool_call)
   - Tool call accumulation (log each accumulate call)
   - Tool execution loop (iteration count, tool calls found)
   - Message construction (log final assistant message)
   - Tool result posting (log result added to history)

4. Add debug logging in `buildContext()`:
   - System prompt included
   - Each message in history (role, content preview, token count)
   - Total token count after building

5. Add debug logging in `buildToolSpecs()`:
   - Number of tools available
   - Each tool name and description

**Key logging points**:
```go
// In runLoop()
debug.Event("AGENT", "Turn started")
debug.Event("AGENT", "Iteration %d", iteration)

messages := a.buildContext()
debug.Request("CONTEXT", "Context built: %d messages, %d tokens", len(messages), a.tracker.TotalTokens())

req := &llm.ChatRequest{...}
debug.JSON("REQUEST", "ChatRequest", req)

eventChan, err := a.client.Stream(ctx, req)
debug.Response("STREAM", "Stream started, err=%v", err)

for event := range eventChan {
    debug.Event("STREAM", "Received event: %s", event.Type)
    // ... existing switch
}

// Tool call execution
debug.Tool("TOOL", "Executing %s with args: %s", toolName, string(finalArgs))
result, err := tool.Execute(finalArgs)
debug.Tool("TOOL", "%s result: err=%v, len=%d", toolName, err, len(result))
```

### Phase 3: Integrate Debug Logger into LLM Client

**Goal**: Log HTTP requests and responses in the LLM client.

**Files to modify**:
- `internal/llm/client.go`

**Tasks**:
1. Add `debug *DebugLogger` field to `Client` struct
2. Add `WithDebug(logger *DebugLogger)` method to Client
3. Add debug logging in `Stream()`:
   - Request URL and method
   - Request headers (sanitize auth headers)
   - Request body (full JSON)
   - Response status code and headers
   - Each SSE event parsed
   - Final usage stats

4. Add debug logging in `buildRequestBody()`:
   - Log the constructed request body before sending

5. Add debug logging in `parseStream()`:
   - Each SSE line received
   - Each StreamEvent created
   - Error conditions

**Key logging points**:
```go
// In Stream()
debug.Request("HTTP", "POST %s", c.BaseURL+chatPath)
debug.JSON("HTTP", "Request body", reqBody)

resp, err := c.httpClient.Do(req)
debug.Response("HTTP", "Response: %s", resp.Status)

// In parseStream()
for scanner.Scan() {
    line := scanner.Text()
    debug.Response("SSE", "Raw line: %s", line)
    // ... parse
    debug.Event("SSE", "Event: type=%s", event.Type)
}
```

### Phase 4: Integrate Debug Logger into Tools

**Goal**: Log tool execution details.

**Files to modify**:
- `internal/tools/registry.go`
- `internal/tools/read.go`
- `internal/tools/write.go`
- `internal/tools/edit.go`
- `internal/tools/bash.go`

**Tasks**:
1. Add `debug *DebugLogger` field to `Registry`
2. Add `WithDebug(logger *DebugLogger)` method to Registry
3. Add debug logging in `Registry.Execute()`:
   - Tool lookup result
   - Input args
   - Execution result (success/error, output length)

4. Add debug logging in each tool's `Execute()`:
   - **read**: path resolved, offset/limit applied, lines read, truncation status
   - **write**: path resolved, parent dirs created, bytes written
   - **edit**: file read, edits applied, diff generated, write result
   - **bash**: command executed, timeout set, output captured, exit code

**Key logging points**:
```go
// In registry.Execute()
debug.Tool("TOOL", "Looking up tool: %s", name)
debug.Tool("TOOL", "Executing %s with args: %s", name, string(args))
result, err := tool.Execute(args)
debug.Tool("TOOL", "%s completed: err=%v, output_len=%d", name, err, len(result))

// In read.Execute()
debug.Tool("READ", "Resolving path: %s", args.path)
debug.Tool("READ", "Reading file: %s (offset=%d, limit=%d)", resolvedPath, offset, limit)
debug.Tool("READ", "Read %d lines, truncated=%v", linesRead, wasTruncated)
```

### Phase 5: CLI Integration

**Goal**: Add `-debug` flag and wire it through the system.

**Files to modify**:
- `cmd/mmok/main.go`
- `internal/app/app.go`
- `internal/app/config_types.go`

**Tasks**:
1. Add `-debug` flag to `cmd/mmok/main.go`
2. Add `Debug` field to `Config` struct in `config_types.go`
3. Pass debug flag through config loading
4. Initialize debug logger when `-debug` is set
5. Pass debug logger to Agent and Client
6. Pass debug logger to Tool Registry

**CLI usage**:
```bash
# Interactive TUI mode with debug logging
./mmok -debug

# Prompt mode with debug logging
./mmok -debug -p "your prompt here" -t 120

# Debug with other flags
./mmok -debug -model gemma4-e4b -endpoint http://localhost:8000/v1 -p "test"
```

### Phase 6: Debug Output Formatting (Optional Enhancement)

**Goal**: Make debug output more readable and filterable.

**Tasks**:
1. Add color coding for different categories (AGENT, REQUEST, TOOL, etc.)
2. Add indentation for nested operations
3. Add verbosity levels (-v, -vv, -vvv)
4. Add category filters (-debug=tools, -debug=agent, etc.)
5. Add log file output option (-debug-log=/path/to/file.log)

## Testing

### Unit Tests
- Test debug logger formatting
- Test JSON pretty-printing
- Test category filtering

### Integration Tests
- Run agent with debug flag, verify log output
- Verify all major code paths produce debug logs
- Verify debug output doesn't break normal operation

### Manual Testing
```bash
# Test basic debug output
./mmok -debug -p "Hello" -t 30

# Test tool calling with debug
./mmok -debug -p "Read the file README.md" -t 60

# Test multi-turn with tools
./mmok -debug -p "Write a file, then read it back" -t 60
```

## Expected Debug Output Example

```
$ ./mmok -debug -p "Read the file AGENTS.md" -t 60

[DEBUG] [2026-04-30 14:23:45] [CONFIG] Debug mode enabled
[DEBUG] [2026-04-30 14:23:45] [AGENT] Creating agent with model=qwen3.5-9b-thinking
[DEBUG] [2026-04-30 14:23:45] [TOOLS] Registry initialized with 4 tools: read, write, edit, bash

[DEBUG] [2026-04-30 14:23:45] [AGENT] Turn started
[DEBUG] [2026-04-30 14:23:45] [AGENT] User message: "Read the file AGENTS.md"

[DEBUG] [2026-04-30 14:23:45] [CONTEXT] Building context
[DEBUG] [2026-04-30 14:23:45] [CONTEXT] System prompt (512 tokens):
  You are a terminal coding agent...
[DEBUG] [2026-04-30 14:23:45] [CONTEXT] Message 1: user (24 tokens): "Read the file AGENTS.md"
[DEBUG] [2026-04-30 14:23:45] [CONTEXT] Total: 2 messages, 536 tokens

[DEBUG] [2026-04-30 14:23:45] [TOOLS] Available tools: 4
[DEBUG] [2026-04-30 14:23:45] [TOOLS] - read: Read file contents with offset/limit support
[DEBUG] [2026-04-30 14:23:45] [TOOLS] - write: Write content to a file
[DEBUG] [2026-04-30 14:23:45] [TOOLS] - edit: Search/replace edits with diff output
[DEBUG] [2026-04-30 14:23:45] [TOOLS] - bash: Execute shell commands

[DEBUG] [2026-04-30 14:23:45] [REQUEST] POST http://localhost:8000/v1/chat/completions
[DEBUG] [2026-04-30 14:23:45] [REQUEST] Headers: Content-Type=application/json, Authorization=[REDACTED]
[DEBUG] [2026-04-30 14:23:45] [REQUEST] Body:
{
  "model": "qwen3.5-9b-thinking",
  "messages": [
    {
      "role": "system",
      "content": "You are a terminal coding agent..."
    },
    {
      "role": "user",
      "content": "Read the file AGENTS.md"
    }
  ],
  "tools": [
    {"type": "function", "function": {...}},
    ...
  ],
  "temperature": 0.7,
  "max_tokens": 2048
}

[DEBUG] [2026-04-30 14:23:46] [RESPONSE] HTTP 200 OK
[DEBUG] [2026-04-30 14:23:46] [RESPONSE] Headers: Content-Type=text/event-stream
[DEBUG] [2026-04-30 14:23:46] [STREAM] Stream started

[DEBUG] [2026-04-30 14:23:46] [EVENT] message_start
[DEBUG] [2026-04-30 14:23:46] [EVENT] thinking_delta: "Okay, I need to read the AGENTS.md file..."
[DEBUG] [2026-04-30 14:23:47] [EVENT] thinking_delta: " I'll use the read tool..."
[DEBUG] [2026-04-30 14:23:47] [EVENT] tool_call_start: id=call_abc123 name=read
[DEBUG] [2026-04-30 14:23:47] [EVENT] tool_call_update: id=call_abc123 args="{\"path\""
[DEBUG] [2026-04-30 14:23:47] [EVENT] tool_call_update: id=call_abc123 args=": \"AGENTS.md\"}"
[DEBUG] [2026-04-30 14:23:47] [EVENT] tool_call_end: id=call_abc123 name=read args="{\"path\": \"AGENTS.md\"}"
[DEBUG] [2026-04-30 14:23:47] [EVENT] message_end: usage={PromptTokens=536 CompletionTokens=87}

[DEBUG] [2026-04-30 14:23:47] [TOOL] Looking up tool: read
[DEBUG] [2026-04-30 14:23:47] [TOOL] Executing read with args: {"path": "AGENTS.md"}
[DEBUG] [2026-04-30 14:23:47] [READ] Resolving path: AGENTS.md
[DEBUG] [2026-04-30 14:23:47] [READ] Resolved path: /Users/remy/docs/code/mmok/AGENTS.md
[DEBUG] [2026-04-30 14:23:47] [READ] Reading file (offset=0, limit=0)
[DEBUG] [2026-04-30 14:23:47] [READ] Read 156 lines (7.2KB), truncated=false
[DEBUG] [2026-04-30 14:23:47] [TOOL] read completed: err=<nil>, output_len=7342

[DEBUG] [2026-04-30 14:23:47] [EVENT] tool_result: id=call_abc123 name=read success=true len=7342
[DEBUG] [2026-04-30 14:23:47] [AGENT] Iteration 1, tool calls executed, continuing loop

[DEBUG] [2026-04-30 14:23:47] [CONTEXT] Building context (iteration 2)
[DEBUG] [2026-04-30 14:23:47] [CONTEXT] Message 3: tool (7342 tokens): read result...
[DEBUG] [2026-04-30 14:23:47] [CONTEXT] Total: 4 messages, 7878 tokens

[DEBUG] [2026-04-30 14:23:48] [REQUEST] POST http://localhost:8000/v1/chat/completions
... (second iteration)

[DEBUG] [2026-04-30 14:23:49] [EVENT] text_delta: "Here's the content of AGENTS.md:"
[DEBUG] [2026-04-30 14:23:50] [EVENT] text_delta: "\n\n# mmok — Terminal Coding Agent..."
[DEBUG] [2026-04-30 14:23:50] [EVENT] message_end: usage={PromptTokens=7878 CompletionTokens=234}
[DEBUG] [2026-04-30 14:23:50] [AGENT] Turn completed successfully

[DEBUG] [2026-04-30 14:23:50] [AGENT] Total tokens: prompt=7878 completion=234
```

## Dependencies

No new dependencies required. Uses standard library:
- `encoding/json` for pretty-printing
- `time` for timestamps
- `fmt` for formatting

## Risks and Considerations

1. **Performance**: Debug logging adds overhead. Ensure it's disabled by default and has minimal impact when off.
2. **Sensitive data**: Be careful not to log bearer tokens or other secrets (already handled by sanitizing headers).
3. **Output volume**: Debug output can be very verbose. Consider adding verbosity levels.
4. **Large tool results**: Tool results (especially file content) can be large. Consider truncating debug output of results.

## Success Criteria

- [ ] `-debug` flag added and working
- [ ] Debug logger outputs to stderr
- [ ] All agent loop iterations logged
- [ ] All LLM requests/responses logged (with JSON formatting)
- [ ] All tool executions logged
- [ ] All events logged
- [ ] Debug output is readable and well-formatted
- [ ] No regression in normal (non-debug) operation
- [ ] Can successfully debug tool calling issues

## Next Steps After Implementation

Once debug mode is working, use it to:
1. Identify bugs in the agent loop
2. Debug tool calling interpretation issues
3. Understand LLM message flow
4. Optimize token usage
5. Diagnose performance issues
