# Plan: Enhanced Prompt/Command Field

## Overview

Enhance the `InputArea` component to support advanced text editing, history navigation, and command autocomplete functionality. This transforms the simple single-line input into a powerful multi-line editor with smart command completion.

## Current State Analysis

The current `InputArea` (`internal/tui/input.go`) provides:
- Single-line text input with basic cursor navigation (left/right)
- Simple history navigation (up/down) when input is empty
- Basic text editing (backspace, delete, Ctrl+W, Ctrl+U, Ctrl+A/E)
- History push/pop with index tracking

**Limitations:**
- No multi-line support (no newline handling for editing)
- No in-text cursor navigation (up/down arrows scroll messages instead)
- History only accessible when at start of empty input
- No command autocomplete for `/` commands
- No value autocomplete for command arguments

## Proposed Architecture

### 1. Multi-line Text Editor Foundation

Transform `InputArea` to support multi-line text with proper cursor positioning:

```go
type InputArea struct {
    // Existing fields
    theme       Theme
    cursorPos   int        // byte position in flattened text
    history     []string
    historyIdx  int
    prompt      string
    width       int
    focused     bool
    blocked     bool

    // New fields for multi-line support
    lines       []string   // slice of lines (rune slices)
    cursorRow   int        // current row (0-indexed)
    cursorCol   int        // current column (0-indexed, rune offset in line)
    scrollOffset int       // vertical scroll offset for view

    // New fields for command autocomplete
    autocomplete       *AutocompleteState
    commandRegistry    map[string]*CommandDefinition
}

type AutocompleteState struct {
    active        bool
    suggestions   []string
    selectedIndex int
    prefix        string    // text before cursor to match against
    insertPos     int       // position where completion will be inserted
    completionType CompletionType // Command, Value, or None
}

type CompletionType int

const (
    CompletionNone CompletionType = iota
    CompletionCommand
    CompletionValue
)

type CommandDefinition struct {
    name        string
    description string
    hasArgs     bool
    argValues   []string  // predefined values for autocomplete
}
```

### 2. Cursor Navigation Enhancements

**New behavior for arrow keys:**

- **Left/Right**: Move cursor within current line (existing behavior)
- **Up/Down**: Move cursor to same column in previous/next line
  - If at column 0 and pressing Up on first line → navigate history up
  - If at column 0 and pressing Down on last line with history → navigate history down
  - If cursor column > line length → clamp to line end
- **Ctrl+A**: Move to start of current line (existing)
- **Ctrl+E**: Move to end of current line (existing)
- **Ctrl+P**: History up (alternative to Up at line start)
- **Ctrl+N**: History down (alternative to Down at line start)
- **Ctrl+F**: Move forward one word
- **Ctrl+B**: Move backward one word

**New behavior for Enter:**
- Without Alt: Submit the entire multi-line text as one message (join lines with spaces or keep newlines)
- With Alt: Insert newline (create new line at cursor position)

### 3. History Navigation

**Enhanced history behavior:**

```
When cursor is at (row=0, col=0):
  - Up: Load previous history entry, place cursor at end
  - Down: Load next history entry (or clear if at most recent), place cursor at end

When cursor is anywhere else:
  - Up/Down: Navigate within current text (line by line)

When history is being navigated:
  - Pressing any navigation key (not up/down) → exit history mode
  - Modifying text → exit history mode
```

**History state tracking:**
```go
type InputArea struct {
    // ...
    historyMode     bool        // true when browsing history
    originalValue   string      // saved value when entering history
    originalCursor  int         // saved cursor position
}
```

### 4. Command Autocomplete System

**Trigger conditions:**
- When text starts with `/`
- When cursor is after `/` and before first space (command completion)
- When cursor is after a complete command (e.g., `/model `) and before space/end (value completion)

**Autocomplete display:**
- Show dropdown/suggestion list below input field
- Highlight current selection
- Show description for commands
- Tab to accept selection
- Escape to dismiss
- Arrow keys to navigate suggestions
- Continue typing to filter suggestions

**Command registration:**
```go
type CommandRegistry struct {
    commands map[string]*CommandDefinition
}

func (r *CommandRegistry) Register(cmd CommandDefinition)
func (r *CommandRegistry) Complete(prefix string) []string
func (r *CommandRegistry) CompleteValues(cmdName, argPrefix string) []string
```

**Built-in commands:**
```
/model <name>      - Change model (autocomplete: available models)
/debug on|off     - Toggle debug mode
/clear            - Clear conversation
/quit | /exit     - Exit application
/help             - Show available commands
```

**Autocomplete UI rendering:**
- Overlay panel below input field
- Max 8-10 suggestions visible
- Scrollable if more suggestions
- Selected item highlighted
- Description shown on right side (if space permits)

### 5. Text Editing Enhancements

**Line operations:**
- Insert newline: Split current line at cursor, create new line
- Delete newline: Join current line with next line at cursor position
- Delete previous newline: Join previous line with current line

**Selection (future enhancement, but design for it):**
- Shift+Arrow: Extend selection
- Ctrl+Shift+Arrow: Extend selection by word
- Delete with selection: Remove selected text

**Word navigation:**
- Ctrl+Left/Alt+B: Move to start of previous word
- Ctrl+Right/Alt+F: Move to end of next word
- Ctrl+Backspace: Delete word before cursor
- Ctrl+Delete: Delete word after cursor

### 6. Integration Points

**TUI Screen (`internal/tui/screen.go`):**
- Add method to render autocomplete dropdown
- Adjust message view height when autocomplete is active
- Pass command registry to input area

**App Model (`internal/app/app.go`):**
- Register commands in `NewAppModel`
- Handle command execution in `handleCommand`
- Pass available models to command registry

**Theme (`internal/tui/theme.go`):**
- Add styles for autocomplete:
  - `AutocompletePanel`: Background panel
  - `AutocompleteItem`: Normal suggestion
  - `AutocompleteItemSelected`: Selected suggestion
  - `AutocompleteDescription`: Command description

### 7. State Machine for Input Mode

```
InputMode states:
  - Normal: Regular text editing
  - History: Browsing history (cursor at row=0, col=0, navigating up/down)
  - Autocomplete: Showing suggestions (Tab triggered, filtering active)
  - Command: After / is typed, waiting for command completion
  - Value: After command + space, waiting for value completion

Transitions:
  Normal → Autocomplete: Type '/' or complete command + space
  Autocomplete → Normal: Escape, Enter, or select item
  Normal → History: At (0,0) and press Up
  History → Normal: Press any key except Up/Down
  Any → Normal: Agent starts running (blocked state)
```

## Implementation Phases

### Phase 1: Multi-line Foundation ✅

**Goal:** Enable multi-line editing with proper cursor navigation

**Tasks:**
1. ✅ Refactor `InputArea` to use `[]string` for lines instead of single `string`
2. ✅ Implement `cursorRow` and `cursorCol` tracking
3. ✅ Update `HandleKey` for Up/Down arrow line navigation
4. ✅ Update `HandleRune` to handle newline insertion
5. ✅ Update `Render` to show multiple lines with scroll
6. ✅ Update `SetValue` to split/flatten lines appropriately
7. ✅ Add `GetLines()` and `SetLines()` methods

**Files to modify:**
- `internal/tui/input.go` (major refactor) ✅
- `internal/tui/screen.go` (update SetDimensions for multi-line) ✅

**Status:** Complete

### Phase 2: Enhanced Navigation & History ✅

**Goal:** Implement smart history navigation and word-based movement

**Tasks:**
1. ✅ Add logic for Up/Down at line start to trigger history
2. ✅ Implement Ctrl+P/N for history navigation
3. ✅ Implement word navigation (Ctrl+Left/Right, Alt+B/F)
4. ✅ Implement word deletion (Ctrl+Backspace, Ctrl+Delete)
5. ✅ Add Home/End for text start/end
6. ✅ Track history mode state
7. ✅ Update `HandleKey` for new key bindings

**Files to modify:**
- `internal/tui/input.go` ✅
- `internal/app/app.go` (update key handling logic) ✅

**Status:** Complete

**New Features:**
- **History Navigation:** Press Up/Down at line start (row=0, col=0) to browse history
- **Ctrl+P/N:** Alternative history navigation (P=previous, N=next)
- **Word Navigation:** Ctrl+Left/Right (or Ctrl+B/F) to move by words
- **Word Deletion:** Ctrl+Backspace (or Ctrl+W/H) to delete word before cursor
- **Text Start/End:** Home/End keys to jump to start/end of all text
- **History Mode:** Tracks when browsing history, exits on non-navigation keys

**Key Bindings Added:**
| Key | Action |
|-----|--------|
| Up at (0,0) | History up |
| Down at (0,0) | History down |
| Ctrl+P | History up |
| Ctrl+N | History down |
| Ctrl+B / Ctrl+Left | Move back one word |
| Ctrl+F / Ctrl+Right | Move forward one word |
| Ctrl+W / Ctrl+H | Delete word before |
| Ctrl+K | Delete to end of line |
| Ctrl+U | Delete from start of line |
| Home | Move to text start |
| End | Move to text end |

### Phase 3: Command Registry & Autocomplete Core

**Goal:** Build autocomplete infrastructure

**Tasks:**
1. Create `CommandDefinition` and `CommandRegistry` types
2. Implement command registration and lookup
3. Implement prefix matching for command completion
4. Implement value completion for specific commands
5. Add `AutocompleteState` to `InputArea`
6. Implement Tab key to trigger autocomplete
7. Implement Escape to dismiss autocomplete
8. Implement arrow key selection within suggestions

**Files to create:**
- `internal/tui/autocomplete.go` (new file for autocomplete logic)

**Files to modify:**
- `internal/tui/input.go`
- `internal/app/app.go` (register commands)

### Phase 4: Autocomplete UI Rendering

**Goal:** Display autocomplete suggestions visually

**Tasks:**
1. Add autocomplete styles to `Theme`
2. Create `AutocompleteView` component (similar to `MessageView`)
3. Implement rendering of suggestion list
4. Integrate autocomplete rendering into `Screen.Render()`
5. Adjust message view height when autocomplete is active
6. Add scroll support for long suggestion lists
7. Show command descriptions in suggestion panel

**Files to create:**
- `internal/tui/autocomplete_view.go` (new file)

**Files to modify:**
- `internal/tui/theme.go`
- `internal/tui/screen.go`
- `internal/tui/input.go` (add RenderAutocomplete method)

### Phase 5: Integration & Polish

**Goal:** Connect all pieces and polish the UX

**Tasks:**
1. Register built-in commands in `NewAppModel`
2. Pass available models to command registry
3. Handle command execution properly (update config, show feedback)
4. Add visual feedback when command is executed
5. Test edge cases:
   - Empty input with history
   - Multi-line input submission
   - Autocomplete with no matches
   - Very long lines
   - Cursor positioning after history load
6. Add keyboard shortcuts documentation
7. Update `PLAN.md` with completion status

**Files to modify:**
- `internal/app/app.go`
- `internal/tui/input.go`
- `internal/tui/screen.go`

## Configuration Options

Add to `Config` struct in `internal/app/config_types.go`:

```go
type Config struct {
    // ... existing fields ...

    // Input behavior
    EnableMultiLine      bool `yaml:"enable_multiline"`      // Enable multi-line editing
    EnableAutocomplete   bool `yaml:"enable_autocomplete"`   // Enable command autocomplete
    AutocompleteMaxItems int  `yaml:"autocomplete_max_items"` // Max suggestions to show
    TabCompletes         bool `yaml:"tab_completes"`         // Enable Tab for completion
}
```

Default values:
- `EnableMultiLine: true`
- `EnableAutocomplete: true`
- `AutocompleteMaxItems: 10`
- `TabCompletes: true`

Environment variables:
- `MMOK_ENABLE_MULTILINE`
- `MMOK_ENABLE_AUTOCOMPLETE`
- `MMOK_AUTOCOMPLETE_MAX_ITEMS`
- `MMOK_TAB_COMPLETES`

## Testing Strategy

### Unit Tests (`internal/tui/input_test.go`)

1. **Cursor navigation tests:**
   - Left/Right movement within line
   - Up/Down movement between lines
   - Word navigation
   - Home/End behavior

2. **History tests:**
   - Up/Down at line start navigates history
   - Modifying text exits history mode
   - History preservation when navigating

3. **Multi-line editing tests:**
   - Newline insertion
   - Line joining on delete
   - SetValue with multi-line text
   - Flatten/expand operations

4. **Autocomplete tests:**
   - Tab triggers autocomplete for `/`
   - Filtering suggestions
   - Selection navigation
   - Accept/ dismiss behavior

### Integration Tests

1. Command execution flow
2. Multi-line message submission
3. History persistence across sessions (future)

## Migration Notes

### Breaking Changes

1. `InputArea.Value()` now returns flattened text (lines joined with `\n`)
2. `InputArea.SetValue(v)` now splits on newlines
3. `HandleKey` returns more `true` values (more keys handled)
4. `Render()` may return multiple lines instead of one

### Backward Compatibility

- Single-line input still works (one line in `[]string`)
- Existing history mechanism preserved
- All existing key bindings maintained
- New features opt-in via config

## Success Criteria

1. ✅ User can navigate text with arrow keys in all directions
2. ✅ History accessible from line start with Up/Down
3. ✅ Multi-line input supported (Alt+Enter for newline)
4. ✅ Command autocomplete triggered by `/`
5. ✅ Value autocomplete for command arguments
6. ✅ Tab to complete, Escape to dismiss
7. ✅ Visual feedback for all operations
8. ✅ No regression in existing functionality
9. ✅ Clean integration with existing codebase
10. ✅ Configurable via YAML/env/flags

## Related Files

- `internal/tui/input.go` - Main input handling (major changes)
- `internal/tui/autocomplete.go` - New file for autocomplete logic
- `internal/tui/autocomplete_view.go` - New file for autocomplete rendering
- `internal/tui/screen.go` - Integration with screen rendering
- `internal/tui/theme.go` - Add autocomplete styles
- `internal/app/app.go` - Command registration and execution
- `internal/app/config_types.go` - Add configuration options

## Future Enhancements

1. **Search within input:** Ctrl+S to search text
2. **Multiple cursors:** Future advanced editing
3. **Snippet expansion:** Type `for` + Tab to expand loop template
4. **Variable substitution:** `${var}` expansion
5. **Command history search:** Ctrl+R for incremental search
6. **Input validation:** Real-time validation for commands
7. **Macro recording:** Record and replay input sequences
