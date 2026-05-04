package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputArea handles user text input with cursor, history, and line editing.
type InputArea struct {
	theme        Theme
	lines        []string // slice of lines (each line is a string)
	cursorRow    int      // current row (0-indexed)
	cursorCol    int      // current column (0-indexed, rune offset in line)
	scrollOffset int      // vertical scroll offset for view
	history      []string
	historyIdx   int
	prompt       string
	width        int
	focused      bool
	blocked      bool

	// History mode state
	historyMode    bool   // true when browsing history
	originalValue  string // saved value when entering history
	originalCursor int    // saved cursor position (byte position in flattened text)
}

// NewInputArea creates a new InputArea.
func NewInputArea(theme Theme, prompt string) *InputArea {
	return &InputArea{
		theme:        theme,
		prompt:       prompt,
		lines:        []string{""},
		cursorRow:    0,
		cursorCol:    0,
		scrollOffset: 0,
		historyIdx:   -1,
		focused:      true,
	}
}

// SetFocused sets whether the input area accepts keystrokes.
func (i *InputArea) SetFocused(focused bool) {
	i.focused = focused
}

// SetBlocked sets whether the input area is visually blocked (agent running).
func (i *InputArea) SetBlocked(blocked bool) {
	i.blocked = blocked
}

// SetValue sets the input value, splitting on newlines.
func (i *InputArea) SetValue(v string) {
	if v == "" {
		i.lines = []string{""}
		i.cursorRow = 0
		i.cursorCol = 0
		return
	}
	i.lines = strings.Split(v, "\n")
	i.cursorRow = len(i.lines) - 1
	if i.cursorRow < 0 {
		i.cursorRow = 0
	}
	i.cursorCol = len([]rune(i.lines[i.cursorRow]))
}

// Value returns the current input value as a flattened string (lines joined with newlines).
func (i *InputArea) Value() string {
	return strings.Join(i.lines, "\n")
}

// GetLines returns the current lines.
func (i *InputArea) GetLines() []string {
	return i.lines
}

// SetLines sets the lines directly.
func (i *InputArea) SetLines(lines []string) {
	if len(lines) == 0 {
		i.lines = []string{""}
	} else {
		i.lines = lines
	}
	i.cursorRow = 0
	i.cursorCol = 0
}

// SetWidth sets the input width.
func (i *InputArea) SetWidth(w int) {
	i.width = w
}

// GetVisibleHeight returns the number of lines currently visible in the input area.
func (i *InputArea) GetVisibleHeight() int {
	if len(i.lines) == 0 {
		return 1
	}
	// Count visible lines (from scrollOffset to the end, limited by maxVisibleLines)
	maxVisibleLines := 10
	displayEnd := len(i.lines)
	if displayEnd-i.scrollOffset > maxVisibleLines {
		displayEnd = i.scrollOffset + maxVisibleLines
	}
	visible := displayEnd - i.scrollOffset
	if visible < 1 {
		visible = 1
	}
	return visible
}

// PushHistory adds the current value to history (if non-empty).
func (i *InputArea) PushHistory() {
	if i.Value() != "" {
		i.history = append(i.history, i.Value())
	}
}

// exitHistoryMode exits history browsing mode and restores original state.
func (i *InputArea) exitHistoryMode() {
	i.historyMode = false
	i.originalValue = ""
	i.originalCursor = 0
}

// enterHistoryMode saves current state and enters history browsing mode.
func (i *InputArea) enterHistoryMode() {
	i.historyMode = true
	i.originalValue = i.Value()
	i.originalCursor = i.cursorCol
}

// getCurrentFlattenedPos returns the current cursor position as a byte offset in flattened text.
func (i *InputArea) getCurrentFlattenedPos() int {
	pos := 0
	for r := 0; r < i.cursorRow; r++ {
		pos += len(i.lines[r]) + 1 // +1 for newline
	}
	pos += i.cursorCol
	return pos
}

// setCursorFromFlattenedPos sets cursorRow and cursorCol from a byte offset in flattened text.
func (i *InputArea) setCursorFromFlattenedPos(pos int) {
	if len(i.lines) == 0 {
		i.cursorRow = 0
		i.cursorCol = 0
		return
	}

	currentPos := 0
	for r, line := range i.lines {
		lineLen := len(line) + 1 // +1 for newline (except last line)
		if currentPos+lineLen > pos || r == len(i.lines)-1 {
			i.cursorRow = r
			i.cursorCol = pos - currentPos
			if i.cursorCol > len(line) {
				i.cursorCol = len(line)
			}
			return
		}
		currentPos += lineLen
	}
	i.cursorRow = len(i.lines) - 1
	i.cursorCol = len(i.lines[i.cursorRow])
}

// navigateHistoryUp loads the previous history entry.
func (i *InputArea) navigateHistoryUp() {
	if len(i.history) == 0 {
		return
	}
	if i.historyIdx < len(i.history)-1 {
		i.historyIdx++
		i.SetValue(i.history[i.historyIdx])
		// Move cursor to end
		i.cursorRow = len(i.lines) - 1
		i.cursorCol = len([]rune(i.lines[i.cursorRow]))
	}
}

// navigateHistoryDown loads the next history entry (or clears if at most recent).
func (i *InputArea) navigateHistoryDown() {
	if len(i.history) == 0 {
		return
	}
	if i.historyIdx >= 0 {
		i.historyIdx--
		if i.historyIdx < 0 {
			// No more history, clear input
			i.SetValue("")
			i.historyIdx = -1
		} else {
			i.SetValue(i.history[i.historyIdx])
		}
		// Move cursor to end
		i.cursorRow = len(i.lines) - 1
		i.cursorCol = len([]rune(i.lines[i.cursorRow]))
	}
}

// moveWordLeft moves cursor back one word.
func (i *InputArea) moveWordLeft() {
	currentLine := i.lines[i.cursorRow]
	runes := []rune(currentLine)

	// First, try to move within current line
	if i.cursorCol > 0 {
		// Skip whitespace going left
		for i.cursorCol > 0 && unicode.IsSpace(runes[i.cursorCol-1]) {
			i.cursorCol--
		}
		// Skip non-whitespace going left
		for i.cursorCol > 0 && !unicode.IsSpace(runes[i.cursorCol-1]) {
			i.cursorCol--
		}
		return
	}

	// Move to previous line and find end of last word
	if i.cursorRow > 0 {
		i.cursorRow--
		i.cursorCol = len([]rune(i.lines[i.cursorRow]))
		i.moveWordLeft() // Recursively find word start
	}
}

// moveWordRight moves cursor forward one word.
func (i *InputArea) moveWordRight() {
	currentLine := i.lines[i.cursorRow]
	runes := []rune(currentLine)
	lineLen := len(runes)

	// First, try to move within current line
	if i.cursorCol < lineLen {
		// Skip non-whitespace going right
		for i.cursorCol < lineLen && !unicode.IsSpace(runes[i.cursorCol]) {
			i.cursorCol++
		}
		// Skip whitespace going right
		for i.cursorCol < lineLen && unicode.IsSpace(runes[i.cursorCol]) {
			i.cursorCol++
		}
		return
	}

	// Move to next line and find start of first word
	if i.cursorRow < len(i.lines)-1 {
		i.cursorRow++
		i.cursorCol = 0
		i.moveWordRight() // Recursively find word end
	}
}

// deleteWordBefore deletes the word before the cursor.
func (i *InputArea) deleteWordBefore() {
	currentLine := i.lines[i.cursorRow]
	runes := []rune(currentLine)

	if i.cursorCol == 0 {
		// Join with previous line if possible
		if i.cursorRow > 0 {
			prevLineLen := len([]rune(i.lines[i.cursorRow-1]))
			i.lines[i.cursorRow-1] = i.lines[i.cursorRow-1] + i.lines[i.cursorRow]
			i.lines = append(i.lines[:i.cursorRow], i.lines[i.cursorRow+1:]...)
			i.cursorRow--
			i.cursorCol = prevLineLen
			i.deleteWordBefore() // Recursively delete word in previous line
		}
		return
	}

	// Skip whitespace going left
	for i.cursorCol > 0 && unicode.IsSpace(runes[i.cursorCol-1]) {
		i.cursorCol--
	}

	// Skip non-whitespace going left
	startCol := i.cursorCol
	for i.cursorCol > 0 && !unicode.IsSpace(runes[i.cursorCol-1]) {
		i.cursorCol--
	}

	// Delete the word
	i.lines[i.cursorRow] = string(runes[:i.cursorCol]) + string(runes[startCol:])
}

// deleteWordAfter deletes the word after the cursor.
func (i *InputArea) deleteWordAfter() {
	currentLine := i.lines[i.cursorRow]
	runes := []rune(currentLine)
	lineLen := len(runes)

	if i.cursorCol >= lineLen {
		// Join with next line if possible
		if i.cursorRow < len(i.lines)-1 {
			i.lines[i.cursorRow] = i.lines[i.cursorRow] + i.lines[i.cursorRow+1]
			i.lines = append(i.lines[:i.cursorRow+1], i.lines[i.cursorRow+2:]...)
			i.deleteWordAfter() // Recursively delete word in next line
		}
		return
	}

	// Skip non-whitespace going right
	endCol := i.cursorCol
	for endCol < lineLen && !unicode.IsSpace(runes[endCol]) {
		endCol++
	}

	// Skip whitespace going right
	for endCol < lineLen && unicode.IsSpace(runes[endCol]) {
		endCol++
	}

	// Delete the word and following whitespace
	i.lines[i.cursorRow] = string(runes[:i.cursorCol]) + string(runes[endCol:])
}

// moveToTextStart moves cursor to the very start of all text.
func (i *InputArea) moveToTextStart() {
	i.cursorRow = 0
	i.cursorCol = 0
}

// moveToTextEnd moves cursor to the very end of all text.
func (i *InputArea) moveToTextEnd() {
	i.cursorRow = len(i.lines) - 1
	if i.cursorRow < 0 {
		i.cursorRow = 0
	}
	i.cursorCol = len([]rune(i.lines[i.cursorRow]))
}

// HandleKey processes key events for the input area.
func (i *InputArea) HandleKey(msg tea.KeyType) (handled bool) {
	if !i.focused {
		return false
	}

	// Check if we should exit history mode
	if i.historyMode {
		switch msg {
		case tea.KeyUp, tea.KeyDown:
			// Stay in history mode for up/down
		default:
			i.exitHistoryMode()
		}
	}

	switch msg {
	case tea.KeyEnter:
		// Insert newline: split current line at cursor, create new line
		currentLine := i.lines[i.cursorRow]
		runes := []rune(currentLine)
		if i.cursorCol >= len(runes) {
			// Cursor at end of line: append empty new line
			i.lines = append(i.lines[:i.cursorRow+1], append([]string{""}, i.lines[i.cursorRow+1:]...)...)
		} else {
			// Cursor in middle of line: split line
			afterCursor := string(runes[i.cursorCol:])
			i.lines[i.cursorRow] = string(runes[:i.cursorCol])
			i.lines = append(i.lines[:i.cursorRow+1], append([]string{afterCursor}, i.lines[i.cursorRow+1:]...)...)
		}
		i.cursorRow++
		i.cursorCol = 0
		return true

	case tea.KeyUp:
		// If at line start (row=0, col=0), navigate history up
		if i.cursorRow == 0 && i.cursorCol == 0 && len(i.history) > 0 {
			if !i.historyMode {
				i.enterHistoryMode()
			}
			i.navigateHistoryUp()
			return true
		}
		// Otherwise, move to previous line
		if i.cursorRow > 0 {
			i.cursorRow--
			targetLineLen := len([]rune(i.lines[i.cursorRow]))
			if i.cursorCol > targetLineLen {
				i.cursorCol = targetLineLen
			}
		}
		return true

	case tea.KeyDown:
		// If at line start and at last line, navigate history down
		if i.cursorRow == len(i.lines)-1 && i.cursorCol == 0 && len(i.history) > 0 {
			if !i.historyMode {
				i.enterHistoryMode()
			}
			i.navigateHistoryDown()
			return true
		}
		// Otherwise, move to next line
		if i.cursorRow < len(i.lines)-1 {
			i.cursorRow++
			targetLineLen := len([]rune(i.lines[i.cursorRow]))
			if i.cursorCol > targetLineLen {
				i.cursorCol = targetLineLen
			}
		}
		return true

	case tea.KeyLeft:
		if i.cursorCol > 0 {
			i.cursorCol--
		} else if i.cursorRow > 0 {
			// Move to end of previous line
			i.cursorRow--
			i.cursorCol = len([]rune(i.lines[i.cursorRow]))
		}
		return true

	case tea.KeyRight:
		currentLineLen := len([]rune(i.lines[i.cursorRow]))
		if i.cursorCol < currentLineLen {
			i.cursorCol++
		} else if i.cursorRow < len(i.lines)-1 {
			// Move to start of next line
			i.cursorRow++
			i.cursorCol = 0
		}
		return true

	case tea.KeyBackspace:
		if i.cursorCol > 0 {
			// Delete character before cursor in current line
			currentLine := i.lines[i.cursorRow]
			runes := []rune(currentLine)
			i.lines[i.cursorRow] = string(runes[:i.cursorCol-1]) + string(runes[i.cursorCol:])
			i.cursorCol--
		} else if i.cursorRow > 0 {
			// Join with previous line
			prevLineLen := len([]rune(i.lines[i.cursorRow-1]))
			i.lines[i.cursorRow-1] = i.lines[i.cursorRow-1] + i.lines[i.cursorRow]
			i.lines = append(i.lines[:i.cursorRow], i.lines[i.cursorRow+1:]...)
			i.cursorRow--
			i.cursorCol = prevLineLen
		}
		return true

	case tea.KeyDelete:
		currentLineLen := len([]rune(i.lines[i.cursorRow]))
		if i.cursorCol < currentLineLen {
			// Delete character at cursor in current line
			currentLine := i.lines[i.cursorRow]
			runes := []rune(currentLine)
			i.lines[i.cursorRow] = string(runes[:i.cursorCol]) + string(runes[i.cursorCol+1:])
		} else if i.cursorRow < len(i.lines)-1 {
			// Join with next line
			i.lines[i.cursorRow] = i.lines[i.cursorRow] + i.lines[i.cursorRow+1]
			i.lines = append(i.lines[:i.cursorRow+1], i.lines[i.cursorRow+2:]...)
			i.cursorCol = currentLineLen
		}
		return true

	case tea.KeyCtrlA: // Home: move to start of current line
		i.cursorCol = 0
		return true

	case tea.KeyCtrlE: // End: move to end of current line
		i.cursorCol = len([]rune(i.lines[i.cursorRow]))
		return true

	case tea.KeyCtrlP: // History up (alternative to Up at line start)
		if len(i.history) > 0 {
			if !i.historyMode {
				i.enterHistoryMode()
			}
			i.navigateHistoryUp()
			return true
		}
		return false

	case tea.KeyCtrlN: // History down (alternative to Down at line start)
		if len(i.history) > 0 {
			if !i.historyMode {
				i.enterHistoryMode()
			}
			i.navigateHistoryDown()
			return true
		}
		return false

	case tea.KeyCtrlB: // Move backward one word
		i.moveWordLeft()
		return true

	case tea.KeyCtrlF: // Move forward one word
		i.moveWordRight()
		return true

	case tea.KeyCtrlH, tea.KeyCtrlW: // Ctrl+H or Ctrl+W: delete word before
		i.deleteWordBefore()
		return true

	case tea.KeyCtrlK: // Ctrl+K: delete from cursor to end of line
		currentLine := i.lines[i.cursorRow]
		runes := []rune(currentLine)
		if i.cursorCol < len(runes) {
			i.lines[i.cursorRow] = string(runes[:i.cursorCol])
		}
		return true

	case tea.KeyCtrlU: // Ctrl+U: delete from start of line to cursor
		currentLine := i.lines[i.cursorRow]
		runes := []rune(currentLine)
		if i.cursorCol > 0 {
			i.lines[i.cursorRow] = string(runes[i.cursorCol:])
			i.cursorCol = 0
		}
		return true

	case tea.KeyHome: // Move to start of text
		i.moveToTextStart()
		return true

	case tea.KeyEnd: // Move to end of text
		i.moveToTextEnd()
		return true
	}

	return false
}

// HandleRune inserts a character at the cursor position.
func (i *InputArea) HandleRune(r rune) {
	if !i.focused {
		return
	}

	// Handle newline character
	if r == '\n' {
		// Insert newline: split current line at cursor, create new line
		currentLine := i.lines[i.cursorRow]
		runes := []rune(currentLine)
		if i.cursorCol >= len(runes) {
			// Cursor at end of line: append empty new line
			i.lines = append(i.lines[:i.cursorRow+1], append([]string{""}, i.lines[i.cursorRow+1:]...)...)
		} else {
			// Cursor in middle of line: split line
			afterCursor := string(runes[i.cursorCol:])
			i.lines[i.cursorRow] = string(runes[:i.cursorCol])
			i.lines = append(i.lines[:i.cursorRow+1], append([]string{afterCursor}, i.lines[i.cursorRow+1:]...)...)
		}
		i.cursorRow++
		i.cursorCol = 0
		return
	}

	// Insert character at cursor position in current line
	currentLine := i.lines[i.cursorRow]
	runes := []rune(currentLine)
	insertPos := i.cursorCol
	i.lines[i.cursorRow] = string(runes[:insertPos]) + string(r) + string(runes[insertPos:])
	i.cursorCol++
}

// Render returns the styled multi-line input.
func (i *InputArea) Render() string {
	if i.blocked {
		return StringsRepeat(" ", i.width)
	}

	prefix := i.theme.InputPrefix.Render(i.prompt)
	prefixWidth := lipgloss.Width(prefix)

	availableWidth := i.width - prefixWidth - 1
	if availableWidth < 10 {
		availableWidth = 10
	}

	// Clamp cursorRow and cursorCol to valid range
	if i.cursorRow >= len(i.lines) {
		i.cursorRow = len(i.lines) - 1
	}
	if i.cursorRow < 0 {
		i.cursorRow = 0
	}
	if i.cursorRow < len(i.lines) {
		lineLen := len([]rune(i.lines[i.cursorRow]))
		if i.cursorCol > lineLen {
			i.cursorCol = lineLen
		}
	}
	if i.cursorCol < 0 {
		i.cursorCol = 0
	}

	// Adjust scroll offset to keep cursor visible
	// We want to show all lines, but scroll if there are too many
	maxVisibleLines := 10 // Max lines to display before scrolling
	if i.cursorRow-i.scrollOffset >= maxVisibleLines {
		i.scrollOffset = i.cursorRow - maxVisibleLines + 1
	}
	if i.scrollOffset > i.cursorRow {
		i.scrollOffset = i.cursorRow
	}

	cursorChar := "▌"
	textStyle := i.theme.InputPrefix.Foreground(lipgloss.Color("144"))
	cursorStyle := textStyle.Copy()

	var lines []string

	// Render all visible lines (from scrollOffset to the end, but limit display height)
	// Show all lines from scrollOffset onwards, regardless of cursor position
	displayEnd := len(i.lines)

	// Limit total lines shown to prevent taking over the screen
	maxLines := maxVisibleLines
	if displayEnd-i.scrollOffset > maxLines {
		displayEnd = i.scrollOffset + maxLines
	}

	for r := i.scrollOffset; r < displayEnd; r++ {
		if r >= len(i.lines) {
			break
		}

		currentLine := i.lines[r]
		runes := []rune(currentLine)
		var before, after string

		// Only show cursor on the current cursor row
		if r == i.cursorRow {
			if i.cursorCol < len(runes) {
				before = string(runes[:i.cursorCol])
				after = string(runes[i.cursorCol:])
			} else {
				before = string(runes)
				after = ""
			}

			// Truncate if too wide
			maxBefore := availableWidth - lipgloss.Width(after) - 1
			if lipgloss.Width(before) > maxBefore && maxBefore > 0 {
				// Truncate from the left
				truncated := before
				for lipgloss.Width(truncated) > maxBefore {
					truncated = truncated[1:]
				}
				before = truncated
			}

			// Render with cursor
			inputLine := prefix + " " + textStyle.Render(before) + cursorStyle.Render(cursorChar) + textStyle.Render(after)

			// Pad to width
			renderedWidth := lipgloss.Width(inputLine)
			if renderedWidth < i.width {
				inputLine += StringsRepeat(" ", i.width-renderedWidth)
			}
			lines = append(lines, inputLine)
		} else {
			// Render line without cursor (just text)
			// Truncate if too wide
			if lipgloss.Width(currentLine) > availableWidth {
				truncated := currentLine
				for lipgloss.Width(truncated) > availableWidth && len(truncated) > 0 {
					truncated = truncated[:len(truncated)-1]
				}
				currentLine = truncated
			}

			// Render without cursor
			inputLine := prefix + " " + textStyle.Render(currentLine)

			// Pad to width
			renderedWidth := lipgloss.Width(inputLine)
			if renderedWidth < i.width {
				inputLine += StringsRepeat(" ", i.width-renderedWidth)
			}
			lines = append(lines, inputLine)
		}
	}

	return strings.Join(lines, "\n")
}

// Input rendering uses shared StringsRepeat from utils.go
