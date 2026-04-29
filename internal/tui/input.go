package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputArea handles user text input with cursor, history, and line editing.
type InputArea struct {
	theme       Theme
	value       string
	cursorPos   int
	history     []string
	historyIdx  int
	prompt      string
	width       int
	focused     bool
}

// NewInputArea creates a new InputArea.
func NewInputArea(theme Theme, prompt string) *InputArea {
	return &InputArea{
		theme:      theme,
		prompt:     prompt,
		historyIdx: -1,
		focused:    true,
	}
}

// SetValue sets the input value.
func (i *InputArea) SetValue(v string) {
	i.value = v
}

// Value returns the current input value.
func (i *InputArea) Value() string {
	return i.value
}

// SetWidth sets the input width.
func (i *InputArea) SetWidth(w int) {
	i.width = w
}

// PushHistory adds the current value to history (if non-empty).
func (i *InputArea) PushHistory() {
	if i.value != "" {
		i.history = append(i.history, i.value)
	}
}

// HandleKey processes key events for the input area.
func (i *InputArea) HandleKey(msg tea.KeyType) (handled bool) {
	if !i.focused {
		return false
	}

	switch msg {
	case tea.KeyEnter:
		return true // Signal to submit

	case tea.KeyUp:
		if len(i.history) == 0 {
			return true
		}
		if i.historyIdx < len(i.history)-1 {
			i.historyIdx++
			i.value = i.history[len(i.history)-1-i.historyIdx]
			i.cursorPos = len(i.value)
		}
		return true

	case tea.KeyDown:
		if i.historyIdx > 0 {
			i.historyIdx--
			i.value = i.history[len(i.history)-1-i.historyIdx]
			i.cursorPos = len(i.value)
		} else {
			i.historyIdx = -1
			i.value = ""
			i.cursorPos = 0
		}
		return true

	case tea.KeyBackspace:
		if i.cursorPos > 0 {
			i.value = i.value[:i.cursorPos-1] + i.value[i.cursorPos:]
			i.cursorPos--
		}
		return true

	case tea.KeyDelete:
		if i.cursorPos < len(i.value) {
			i.value = i.value[:i.cursorPos] + i.value[i.cursorPos+1:]
		}
		return true

	case tea.KeyLeft:
		if i.cursorPos > 0 {
			i.cursorPos--
		}
		return true

	case tea.KeyRight:
		if i.cursorPos < len(i.value) {
			i.cursorPos++
		}
		return true

	case tea.KeyCtrlA: // Home
		i.cursorPos = 0
		return true

	case tea.KeyCtrlE: // End
		i.cursorPos = len(i.value)
		return true

	case tea.KeyCtrlW: // Delete word backward
		start := i.cursorPos
		for start > 0 && i.value[start-1] == ' ' {
			start--
		}
		for start > 0 && i.value[start-1] != ' ' {
			start--
		}
		i.value = i.value[:start] + i.value[i.cursorPos:]
		i.cursorPos = start
		return true

	case tea.KeyCtrlU: // Delete line
		i.value = ""
		i.cursorPos = 0
		return true
	}

	return false
}

// Handle Rune inserts a character at the cursor position.
func (i *InputArea) HandleRune(r rune) {
	if !i.focused {
		return
	}
	insertPos := i.cursorPos
	i.value = i.value[:insertPos] + string(r) + i.value[insertPos:]
	i.cursorPos++
}

// Render returns the styled input line.
func (i *InputArea) Render() string {
	prefix := i.theme.InputPrefix.Render(i.prompt)
	prefixWidth := lipgloss.Width(prefix)

	availableWidth := i.width - prefixWidth - 1
	if availableWidth < 10 {
		availableWidth = 10
	}

	// Split value into before/after cursor
	before := i.value[:i.cursorPos]
	after := i.value[i.cursorPos:]

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
	cursorChar := "▌"
	cursorStyle := i.theme.InputPrefix.Foreground(lipgloss.Color("144"))

	text := before + after
	inputLine := prefix + " " + text + " " + cursorStyle.Render(cursorChar)

	// Pad to width
	renderedWidth := lipgloss.Width(inputLine)
	if renderedWidth < i.width {
		inputLine += StringsRepeat(" ", i.width-renderedWidth)
	}

	return inputLine
}

// Input rendering uses shared StringsRepeat from utils.go
