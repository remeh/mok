# PLAN: Visual Diff Display for Edit Tool

## Overview

Enhance the edit tool result display with a visual, colorized unified diff that can be expanded/collapsed in the TUI. This provides immediate visibility into code changes without requiring users to read raw diff output.

## Goals

- Display edit tool results as formatted, colorized unified diffs
- Support expand/collapse toggle for diff output
- Show diff statistics (+N -M lines) in collapsed state
- Preserve ANSI color codes for syntax highlighting in diffs
- Maintain compatibility with existing message rendering pipeline

## Non-Goals

- Inline diff highlighting within the original file content
- Side-by-side diff view
- Diff visualization for non-edit tool results
- Git-style diff with file headers (keep it simple)

## Technical Approach

### Diff Formatting Strategy

Use `go-diff`'s `DiffPrettyText()` instead of `PatchToText()`:
- `DiffPrettyText()` produces ANSI-colored output suitable for TUI
- Green (`\x1b[32m`) for additions
- Red (`\x1b[31m`) for deletions  
- No URL encoding, clean text output
- Preserves line structure for better readability

### Rendering Strategy

1. **Collapsed state**: Show summary with line counts
   - Format: `✓ edit: +3 -2 lines` or `✓ edit: 5 lines changed`
   - Click to expand

2. **Expanded state**: Render full diff with ANSI colors
   - Use existing markdown renderer (test ANSI passthrough)
   - If markdown renderer strips colors, use custom diff rendering
   - Preserve word wrapping for wrapped content

3. **Toggle mechanism**: Reuse existing collapsed/expanded pattern
   - Add `DiffExpanded` field to `types.Message`
   - Click handler toggles expansion state

## Implementation Steps

### Phase 1: Core Diff Formatting (edit.go)

**File**: `internal/tools/edit.go`

**Changes**:
1. Replace `PatchToText()` with `DiffPrettyText()`
2. Add helper function to parse diff statistics (+/- line counts)
3. Update return format to include parsed metadata

**Code outline**:
```go
// Generate pretty diff with ANSI colors
dmp := diffmatchpatch.New()
diffs := dmp.DiffMain(originalStr, modified, false)
unifiedDiff := dmp.DiffPrettyText(diffs)

// Extract stats for summary
stats := parseDiffStats(unifiedDiff)
// stats.Additions, stats.Deletions

// Return formatted result
return fmt.Sprintf("Applied edit to %s\n\n%s", resolved, unifiedDiff), nil
```

**New helper function**:
```go
type DiffStats struct {
    Additions int
    Deletions int
}

func parseDiffStats(diffText string) DiffStats {
    // Count lines starting with + (but not +++)
    // Count lines starting with - (but not ---)
    // Return stats
}
```

### Phase 2: Message Type Extensions (types/message.go)

**File**: `internal/types/message.go`

**Changes**:
1. Add `DiffExpanded` field to `Message` struct
2. Add `DiffStats` field to store pre-parsed statistics
3. Update `NewToolResult` to accept and store diff stats
4. Update `generateSummary` to use diff stats for edit tool

**Code outline**:
```go
type Message struct {
    // ... existing fields ...
    DiffExpanded bool        // When true, show full diff
    DiffStats    DiffStats   // Pre-parsed +N -M counts
}

// NewToolResult with diff stats support
func NewToolResult(toolName, result string, isError bool, stats DiffStats) *Message {
    return &Message{
        // ... existing fields ...
        Collapsed:   true,
        DiffExpanded: false,
        DiffStats:   stats,
        Summary:     generateSummary(toolName, result, isError, stats),
    }
}
```

### Phase 3: Diff Statistics Parsing (tools/diff_stats.go)

**File**: `internal/tools/diff_stats.go` (new file)

**Purpose**: Extract statistics from unified diff output

**Content**:
```go
package tools

import (
    "strings"
    "regexp"
)

// DiffStats holds line change counts from a unified diff
type DiffStats struct {
    Additions int
    Deletions int
}

// ParseDiffStats extracts + and - line counts from unified diff
func ParseDiffStats(diffText string) DiffStats {
    stats := DiffStats{}
    
    lines := strings.Split(diffText, "\n")
    for _, line := range lines {
        // Skip hunk headers like @@ -1,3 +1,4 @@
        if strings.HasPrefix(line, "@@") {
            continue
        }
        
        if len(line) > 0 && line[0] == '+' && line[1] != '+' {
            stats.Additions++
        } else if len(line) > 0 && line[0] == '-' && line[1] != '-' {
            stats.Deletions++
        }
    }
    
    return stats
}

// FormatStats returns human-readable summary
func (s DiffStats) FormatStats() string {
    if s.Additions == 0 && s.Deletions == 0 {
        return "no changes"
    }
    
    var parts []string
    if s.Additions > 0 {
        parts = append(parts, fmt.Sprintf("+%d", s.Additions))
    }
    if s.Deletions > 0 {
        parts = append(parts, fmt.Sprintf("-%d", s.Deletions))
    }
    
    return strings.Join(parts, " ")
}
```

### Phase 4: Message View Diff Rendering (tui/message_view.go)

**File**: `internal/tui/message_view.go`

**Changes**:
1. Add `DiffExpanded` to fingerprint calculation
2. Update `renderMessageLines` to handle diff rendering
3. Add conditional rendering for expanded vs collapsed diffs

**Code outline**:
```go
func (v *MessageView) computeFingerprint(msg *types.Message) string {
    return fmt.Sprintf("%s|%d|%t|%t|%t|%t|%t|%t|%s|%s|%s|%s|%s",
        msg.Type, v.width,
        msg.ThinkingExpanded, msg.Streaming, msg.Collapsed, msg.IsError,
        msg.DiffExpanded, msg.ToolName, msg.ToolArgs, msg.Summary,
        msg.ThinkingText, msg.Content,
    )
}

func (v *MessageView) renderMessageLines(msg *types.Message) []string {
    // ... existing code ...
    
    case types.MsgToolResult:
        if msg.Collapsed && msg.Summary != "" {
            tag := v.tagStyle(msg).Render("[" + msg.ToolName + "]")
            rest := v.theme.Dim.Render(" " + msg.Summary + "  (click to expand)")
            lines = append(lines, "  "+tag+rest)
            return lines
        }
        
        // For expanded edit diffs, render with special handling
        if msg.ToolName == "edit" && msg.DiffExpanded {
            return v.renderDiff(msg.Content, msg.DiffStats)
        }
        
        content = msg.Content
```

**New diff rendering function**:
```go
func (v *MessageView) renderDiff(diffText string, stats DiffStats) []string {
    var lines []string
    
    // Add stats header
    statsLine := fmt.Sprintf("  %s", v.theme.Dim.Render(
        fmt.Sprintf("edit: %s", stats.FormatStats())))
    lines = append(lines, statsLine)
    lines = append(lines, "")
    
    // Render diff content (preserve ANSI codes)
    // Option 1: Pass through markdown renderer
    // Option 2: Custom rendering that preserves ANSI
    
    // For now, try markdown renderer
    if v.mdRenderer != nil {
        if styled, err := v.mdRenderer.Render(diffText); err == nil {
            wrapped := wordwrap.String(styled, v.width-2)
            for _, line := range strings.Split(wrapped, "\n") {
                lines = append(lines, line)
            }
            return lines
        }
    }
    
    // Fallback: plain rendering (may lose colors)
    wrapped := wordwrap.String(diffText, v.width-2)
    for _, line := range strings.Split(wrapped, "\n") {
        lines = append(lines, v.theme.ToolResult.Render(line))
    }
    
    return lines
}
```

### Phase 5: Theme Extensions (tui/theme.go)

**File**: `internal/tui/theme.go`

**Changes**:
1. Add `DiffAdd` style (green)
2. Add `DiffRemove` style (red)
3. These may be auto-applied by ANSI codes, but explicit styles help

**Code outline**:
```go
type Theme struct {
    // ... existing fields ...
    DiffAdd     lipgloss.Style
    DiffRemove  lipgloss.Style
}

func DefaultTheme() Theme {
    return Theme{
        // ... existing fields ...
        DiffAdd:     lipgloss.NewStyle().Foreground(lipgloss.Color("42")),  // green
        DiffRemove:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")), // red
    }
}
```

### Phase 6: Click Handler Updates (tui/screen.go or tui/input.go)

**File**: `internal/tui/screen.go` (or appropriate file)

**Changes**:
1. Update click handler to toggle `DiffExpanded` for edit tool results
2. Ensure collapsed messages can be expanded on click

**Code outline**:
```go
func (m Model) handleMouseClick(y int) {
    msgIndex := m.messageView.MessageAtY(y)
    if msgIndex < 0 {
        return
    }
    
    msg := m.messages[msgIndex]
    
    // Handle tool result expansion
    if msg.Type == types.MsgToolResult && !msg.Collapsed {
        // Toggle expansion for edit diffs
        if msg.ToolName == "edit" {
            msg.DiffExpanded = !msg.DiffExpanded
            m.messageView.SetMessages(m.messages)
        }
    }
}
```

### Phase 7: Agent Loop Integration (internal/agent/loop.go)

**File**: `internal/agent/loop.go`

**Changes**:
1. Update tool result creation to include diff stats
2. Pass stats to `NewToolResult`

**Code outline**:
```go
// After tool execution
if result, err := tool.Execute(args); err == nil {
    stats := tools.ParseDiffStats(result)
    msg := types.NewToolResult(toolName, result, false, stats)
    agent.AddMessage(msg)
    m.eventCh <- events.EventToolResult{
        ToolName: toolName,
        Result:   result,
    }
}
```

### Phase 8: Testing

**Test cases**:
1. Edit with single line change
2. Edit with multiple line changes
3. Edit with additions only
4. Edit with deletions only
5. Edit with mixed changes
6. Empty edit (no changes)
7. Large file edit (ensure rendering doesn't hang)
8. ANSI color passthrough in markdown renderer

**Test file**: `internal/tools/diff_stats_test.go`

## UI/UX Details

### Collapsed State
```
  [edit] ✓ edit: +3 -2 lines  (click to expand)
```

### Expanded State
```
  [edit] edit: +3 -2 lines

@@ -5,13 +5,19 @@
  func main() {
-	fmt.Println("hello")
+	fmt.Println("hello world")
+	fmt.Println("foo")
  }
```

### Keyboard Interactions
- `Enter` when focused on tool result: expand/collapse
- Arrow keys: navigate between messages
- `g` + `G`: scroll to bottom (existing)

## Edge Cases to Handle

1. **Very large diffs**: Truncate display with "show more" option
2. **Binary files**: Show "binary file changed" message
3. **No changes detected**: Show "no changes made" message
4. **ANSI stripping**: Fallback to plain text if markdown renderer strips colors
5. **Unicode in diffs**: Ensure proper handling of non-ASCII characters

## Dependencies

- **Existing**: `github.com/sergi/go-diff/diffmatchpatch` (already imported)
- **No new dependencies required**

## Estimated Timeline

- Phase 1: 30 min
- Phase 2: 30 min
- Phase 3: 30 min
- Phase 4: 2 hours
- Phase 5: 15 min
- Phase 6: 30 min
- Phase 7: 30 min
- Phase 8: 1 hour
- **Total**: ~4 hours

## Success Criteria

1. Edit tool results show formatted diff when expanded
2. Diff statistics are accurate (+N -M counts)
3. ANSI colors render correctly in TUI
4. Expand/collapse toggle works reliably
5. No performance degradation with large diffs
6. All existing tests still pass

## Future Enhancements (Not in Scope)

- Inline diff highlighting (showing changes in context)
- Diff navigation (jump to next/previous hunk)
- Syntax highlighting for specific file types
- Side-by-side diff view
- Export diff to file
- Diff stash/undo functionality
