# ui.log Implementation Analysis

## Executive Summary

**Difficulty: Moderate** (Estimated: 1-2 days of implementation)

Writing a `ui.log` file that captures every message exactly as the user sees it in the TUI is **feasible and straightforward**. The codebase has a clear separation between:
- **Data model** (`types.Message` with full content)
- **Rendering logic** (`tui.MessageView` with collapsed/expanded states)
- **Event-driven architecture** (agent events stream to the UI)

The main challenge is capturing the **final rendered state** (with styling) vs. the **raw content** (without styling).

---

## Current Architecture Overview

### Data Flow
```
Agent Loop → Events (events.go) → AppModel.handleAgentEvent() → 
types.Message → MessageView.Render() → TUI Output
```

### Key Components

#### 1. **Message Storage** (`internal/app/app.go`)
- `m.Messages []*types.Message` - stores all messages in memory
- Messages are appended on events (user input, assistant response, tool calls/results)
- Each message has full content available

#### 2. **Message Type** (`internal/types/message.go`)
```go
type Message struct {
    Type         MessageType  // user, assistant, tool_call, tool_result, system
    Content      string       // Full content (what LLM sees)
    Summary      string       // One-line display summary for tool results
    ThinkingText string       // Reasoning/thinking text
    ToolName     string
    ToolArgs     string
    IsError      bool
    Streaming    bool
    Collapsed    bool      // When true, show Summary instead of Content
    ThinkingExpanded bool   // When true, show full thinking text
}
```

**Important**: The `Collapsed` and `ThinkingExpanded` fields are **user-controlled UI states**, not data states. The full content is always available in `Content` and `ThinkingText`.

#### 3. **Rendering Logic** (`internal/tui/message_view.go`)
- `renderMessageLines()` - renders each message into styled lines
- Collapsed tool results show only `Summary` with "(click to expand)" hint
- Collapsed thinking shows only `[thinking] (click to expand)`
- Streaming messages append `▌` cursor character
- Markdown rendering for assistant messages (via glamour)

#### 4. **Event System** (`internal/agent/events.go`)
Events emitted during a turn:
- `EventTurnStart`
- `EventMessageStart` - starts assistant message
- `EventTextDelta` - streaming text chunks
- `EventThinkingDelta` - streaming thinking chunks
- `EventMessageEnd` - assistant message complete
- `EventToolCallStart` - tool call begins
- `EventToolCallUpdate` - tool args streaming
- `EventToolCallEnd` - tool call complete
- `EventToolResult` - tool execution result
- `EventTurnEnd` - turn complete

---

## Implementation Approaches

### Approach 1: Post-Session Export (Recommended)

**When**: User types `/export` or presses a key (e.g., `Ctrl+L`) at the end of a session.

**Pros**:
- Simple to implement
- No performance overhead during session
- Captures final state (no streaming artifacts)

**Cons**:
- Not real-time
- User must remember to export

**Implementation Steps**:

1. **Add export command** in `app.go`:
```go
case "export":
    if err := m.exportToLogFile("ui.log"); err != nil {
        m.Screen.SetStatusMessage(fmt.Sprintf("Export failed: %v", err))
        return nil
    }
    m.Screen.SetStatusMessage("Exported to ui.log")
    return nil
```

2. **Implement export function** in `app.go`:
```go
func (m *AppModel) exportToLogFile(filename string) error {
    f, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer f.Close()
    
    // Write header
    fmt.Fprintf(f, "=== mmok TUI Session Log ===\n")
    fmt.Fprintf(f, "Model: %s\n", m.Config.Model)\n")
    fmt.Fprintf(f, "Started: %s\n\n", time.Now().Format(time.RFC3339))
    
    // Write each message without collapsing
    for _, msg := range m.Messages {
        // Reset UI state flags before writing
        msg.Collapsed = false
        msg.ThinkingExpanded = true
        
        // Write message header
        switch msg.Type {
        case types.MsgUser:
            fmt.Fprintf(f, "\n--- USER ---\n")
        case types.MsgAssistant:
            fmt.Fprintf(f, "\n--- ASSISTANT ---\n")
        case types.MsgToolCall:
            fmt.Fprintf(f, "\n--- TOOL CALL: %s ---\n", msg.ToolName)
        case types.MsgToolResult:
            fmt.Fprintf(f, "\n--- TOOL RESULT: %s ---\n", msg.ToolName)
            if msg.IsError {
                fmt.Fprint(f, "[ERROR]\n")
            }
        case types.MsgSystem:
            fmt.Fprintf(f, "\n--- SYSTEM ---\n")
        }
        
        // Write thinking text first (if any)
        if msg.ThinkingText != "" {
            fmt.Fprintf(f, "[thinking]\n%s\n[/thinking]\n\n", msg.ThinkingText)
        }
        
        // Write content
        fmt.Fprintf(f, "%s\n", msg.Content)
        
        // Write tool-specific info
        if msg.Type == types.MsgToolCall {
            fmt.Fprintf(f, "Args:\n%s\n", msg.ToolArgs)
        }
        if msg.Type == types.MsgToolResult {
            fmt.Fprintf(f, "Result:\n%s\n", msg.Content)
        }
    }
    
    // Write footer
    fmt.Fprintf(f, "\n=== Session Complete ===\n")
    fmt.Fprintf(f, "Total messages: %d\n", len(m.Messages))
    
    return nil
}
```

3. **Add key binding** (optional):
```go
case tea.KeyCtrlL:
    if m.agentRunning {
        break
    }
    if cmd := m.exportToLogFile("ui.log"); cmd != nil {
        return m, cmd
    }
```

---

### Approach 2: Real-Time Logging During Session

**When**: Log every event as it happens, similar to debug logging.

**Pros**:
- Captures streaming progress
- Can be enabled/disabled dynamically
- Useful for debugging

**Cons**:
- Slight performance overhead
- Log includes intermediate states (streaming deltas)
- Requires careful event handling

**Implementation Steps**:

1. **Add log file initialization** in `NewAppModel()`:
```go
var logFile *os.File
if cfg.Debug {
    debug = agent.NewDebugLoggerFile(true, "debug.log")
    // Also create ui.log
    logFile, err = os.Create("ui.log")
    if err != nil {
        return nil, fmt.Errorf("failed to create ui.log: %w", err)
    }
    fmt.Fprintln(logFile, "=== mmok TUI Session Log ===")
    fmt.Fprintf(logFile, "Started: %s\n\n", time.Now().Format(time.RFC3339))
}
```

2. **Add log writer to AppModel**:
```go
type AppModel struct {
    // ... existing fields ...
    uiLogFile *os.File
}
```

3. **Log events in `handleAgentEvent()`**:
```go
func (m *AppModel) handleAgentEvent(event agent.Event) {
    // ... existing switch ...
    
    case agent.EventMessageStart:
        // ... existing code ...
        if m.uiLogFile != nil {
            fmt.Fprintln(m.uiLogFile, "\n--- ASSISTANT MESSAGE START ---")
        }
    
    case agent.EventTextDelta:
        if m.uiLogFile != nil {
            fmt.Fprint(m.uiLogFile, ev.Text)
        }
        // ... existing code ...
    
    case agent.EventThinkingDelta:
        if m.uiLogFile != nil {
            fmt.Fprint(m.uiLogFile, ev.Text)
        }
        // ... existing code ...
    
    case agent.EventMessageEnd:
        if m.uiLogFile != nil {
            fmt.Fprintln(m.uiLogFile, "\n--- ASSISTANT MESSAGE END ---")
        }
        // ... existing code ...
    
    case agent.EventToolCallStart:
        if m.uiLogFile != nil {
            fmt.Fprintf(m.uiLogFile, "\n--- TOOL CALL: %s (start) ---\n", ev.Name)
        }
        // ... existing code ...
    
    case agent.EventToolCallUpdate:
        if m.uiLogFile != nil {
            fmt.Fprintf(m.uiLogFile, "  Args update: %s\n", ev.RawArgs)
        }
        // ... existing code ...
    
    case agent.EventToolCallEnd:
        if m.uiLogFile != nil {
            fmt.Fprintf(m.uiLogFile, "\n--- TOOL CALL: %s (end) ---\n", ev.Name)
        }
        // ... existing code ...
    
    case agent.EventToolResult:
        if m.uiLogFile != nil {
            fmt.Fprintf(m.uiLogFile, "\n--- TOOL RESULT: %s ---\n", ev.Name)
            if ev.IsError {
                fmt.Fprintln(m.uiLogFile, "[ERROR]")
            }
            fmt.Fprintf(m.uiLogFile, "Result:\n%s\n", ev.Result)
        }
        // ... existing code ...
}
```

4. **Close log file on quit**:
```go
case tea.KeyCtrlC:
    // ... existing code ...
    if m.uiLogFile != nil {
        fmt.Fprintln(m.uiLogFile, "\n=== Session Complete ===")
        m.uiLogFile.Close()
    }
    return m, tea.Quit
```

---

### Approach 3: Hybrid (Real-Time + Final Cleanup)

**When**: Log events in real-time but rewrite the file at session end to remove streaming artifacts and ensure clean output.

**Pros**:
- Real-time visibility
- Clean final output
- Best of both approaches

**Cons**:
- More complex implementation
- Two-pass writing

**Implementation**: Combine approaches 1 and 2, with a final rewrite step.

---

## Capturing Styled Output (Advanced)

If you want to capture **exact visual output** (with colors, markdown rendering, etc.), you need to:

1. **Capture rendered lines** from `MessageView.Render()`
2. **Strip ANSI color codes** if you want plain text
3. **Keep ANSI codes** if you want colored output

### Option A: Plain Text (No Colors)

Use `glamour`'s markdown renderer to get styled output, then strip ANSI codes:

```go
import "github.com/mattn/go-isatty"
import "github.com/xhit/go-str2duration/v2"

// Helper to strip ANSI codes
func stripANSI(s string) string {
    // Use regexp or library to remove ANSI escape sequences
    // Example: \x1b\[[0-9;]*[a-zA-Z]
    return ansi.Strip(s) // from github.com/acarl005/ansi
}
```

### Option B: Colored Output (ANSI codes preserved)

Simply capture `Screen.Render()` output and write to file. However, this mixes:
- Message content
- Input line
- Status bar
- Scroll hints

You'd need to parse the output to extract just messages, which is fragile.

**Recommendation**: Use the data model (Approach 1 or 2) and optionally add markdown rendering separately.

---

## Challenges and Considerations

### 1. **Streaming Artifacts**
- Real-time logging will capture partial text chunks
- Solution: Buffer and write on `EventMessageEnd` instead

### 2. **Markdown Rendering**
- Assistant messages are rendered via glamour (markdown → styled HTML)
- If you want markdown source, capture `msg.Content` before rendering
- If you want rendered output, use glamour renderer directly

### 3. **User Input History**
- Current implementation doesn't log user input separately
- Solution: Log when user submits (in `submitMessage()`)

### 4. **Tool Call Arguments**
- Tool args stream in chunks (`EventToolCallUpdate`)
- Store full args in `msg.ToolArgs` by `EventToolCallEnd`
- Log from `msg.ToolArgs` after completion

### 5. **Thinking/Reasoning Text**
- Currently collapsed by default in TUI
- Log should always expand thinking text (as it's part of the response)
- Set `ThinkingExpanded = true` before writing

### 6. **Tool Result Collapsing**
- Tool results are collapsed by default with summary
- Log should always expand (show full result)
- Set `Collapsed = false` before writing

### 7. **Timestamps**
- Messages have `Timestamp` field but it's not displayed in TUI
- Consider adding timestamps to log for reference

### 8. **File Size**
- Long sessions could produce large log files
- Consider rotation or truncation for very long sessions

---

## Recommended Implementation Plan

### Phase 1: Basic Export (1-2 hours)
1. Add `/export` command
2. Implement `exportToLogFile()` that writes all messages without collapsing
3. Test with simple conversation

### Phase 2: Real-Time Logging (2-3 hours)
1. Add optional real-time logging flag (`--log-ui`)
2. Log events as they happen (buffered to avoid streaming artifacts)
3. Add cleanup step at session end

### Phase 3: Enhanced Formatting (1-2 hours)
1. Add markdown rendering to log (optional)
2. Add timestamps
3. Add session metadata (model, endpoint, config)
4. Add statistics (token usage, tool calls, etc.)

### Phase 4: Advanced Features (optional)
1. Colored output (ANSI codes)
2. Log rotation for long sessions
3. Export to different formats (JSON, Markdown, HTML)
4. Search/filter in log viewer

---

## Code Locations to Modify

| File | Change |
|------|--------|
| `internal/app/app.go` | Add export command, export function, log file handling |
| `cmd/mmok/main.go` | Add CLI flag for log file path (optional) |
| `internal/types/message.go` | Consider adding `LogTimestamp` field (optional) |

---

## Testing Strategy

1. **Unit tests** for export function:
   - Verify all message types are logged correctly
   - Verify collapsed states are expanded
   - Verify thinking text is included

2. **Integration tests**:
   - Run a full session with tool calls
   - Verify log file matches TUI output
   - Verify no data loss

3. **Manual testing**:
   - Test with long conversations
   - Test with large tool outputs
   - Test with markdown content
   - Test with errors

---

## Conclusion

**Difficulty Assessment: Moderate**

The codebase is well-structured for this feature:
- ✅ Clear separation of data and rendering
- ✅ Complete message content available in memory
- ✅ Event-driven architecture for real-time logging
- ✅ Existing debug logging pattern to follow

**Estimated effort**: 1-2 days for a complete implementation with both export and real-time logging.

**Key advantage**: No major refactoring needed. The feature can be added incrementally with minimal risk to existing functionality.

---

## Quick Start Example

Here's a minimal implementation you can add to `app.go` right now:

```go
// Add to AppModel struct
type AppModel struct {
    // ... existing fields ...
    uiLogFile *os.File
}

// Add to NewAppModel()
func NewAppModel(cfg *Config) (*AppModel, error) {
    // ... existing code ...
    
    // Create ui.log file
    logFile, err := os.Create("ui.log")
    if err != nil {
        return nil, fmt.Errorf("failed to create ui.log: %w", err)
    }
    fmt.Fprintln(logFile, "=== mmok TUI Session Log ===")
    fmt.Fprintf(logFile, "Model: %s\n", cfg.Model)
    fmt.Fprintf(logFile, "Started: %s\n\n", time.Now().Format(time.RFC3339))
    
    return &AppModel{
        // ... existing fields ...
        uiLogFile: logFile,
    }, nil
}

// Add export method
func (m *AppModel) exportToLogFile() error {
    if m.uiLogFile == nil {
        return fmt.Errorf("logging not enabled")
    }
    
    for _, msg := range m.Messages {
        // Expand all collapsed content
        msg.Collapsed = false
        msg.ThinkingExpanded = true
        
        switch msg.Type {
        case types.MsgUser:
            fmt.Fprintln(m.uiLogFile, "\n--- USER ---")
        case types.MsgAssistant:
            fmt.Fprintln(m.uiLogFile, "\n--- ASSISTANT ---")
        case types.MsgToolCall:
            fmt.Fprintf(m.uiLogFile, "\n--- TOOL CALL: %s ---\n", msg.ToolName)
        case types.MsgToolResult:
            fmt.Fprintf(m.uiLogFile, "\n--- TOOL RESULT: %s ---\n", msg.ToolName)
            if msg.IsError {
                fmt.Fprintln(m.uiLogFile, "[ERROR]")
            }
        }
        
        if msg.ThinkingText != "" {
            fmt.Fprintln(m.uiLogFile, "[thinking]")
            fmt.Fprintln(m.uiLogFile, msg.ThinkingText)
            fmt.Fprintln(m.uiLogFile, "[/thinking]")
        }
        
        fmt.Fprintln(m.uiLogFile, msg.Content)
    }
    
    fmt.Fprintln(m.uiLogFile, "\n=== Session Complete ===")
    return nil
}

// Add to handleAgentEvent() - log on turn end
case agent.EventTurnEnd:
    // ... existing code ...
    if m.uiLogFile != nil {
        m.exportToLogFile()
    }
```

This minimal version provides basic logging with no streaming artifacts and captures all content as the user sees it (expanded).
