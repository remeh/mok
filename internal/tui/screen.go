package tui

import (
	"strings"

	"github.com/user/mok/internal/types"
)

// Screen composes all TUI components into the main layout.
type Screen struct {
	theme     Theme
	msgView   *MessageView
	inputArea *InputArea
	statusBar *StatusBar
	autocompleteView *AutocompleteView
	width     int
	height    int
	streaming bool
}

// NewScreen creates a new Screen with all sub-components.
func NewScreen(theme Theme) *Screen {
	return &Screen{
		theme:            theme,
		msgView:          NewMessageView(theme),
		inputArea:        NewInputArea(theme, ">"),
		statusBar:        NewStatusBar(theme),
		autocompleteView: NewAutocompleteView(theme),
	}
}

// SetDimensions updates all component dimensions.
//
// Layout: the message view takes remaining space, the input area grows
// with its content (up to a max), and the status bar always claims 1 line.
// The scroll-position indicator lives inside the status bar (↓N segment).
// If autocomplete is active, it reduces the message view height to show suggestions.
func (s *Screen) SetDimensions(w, h int) {
	s.width = w
	s.height = h

	// Calculate input area height based on number of lines
	inputHeight := s.inputArea.GetVisibleHeight()
	if inputHeight < 1 {
		inputHeight = 1
	}
	if inputHeight > 10 {
		inputHeight = 10 // Cap input height
	}

	// Check if autocomplete is active
	autocompleteHeight := 0
	if s.inputArea.AutocompleteIsActive() {
		autocompleteState := s.inputArea.GetAutocompleteState()
		prefix := autocompleteState.GetPrefix()
		autocompleteHeight = s.autocompleteView.GetHeight(autocompleteState, prefix)
	}

	// Status bar takes 1 line, input takes inputHeight lines
	// If autocomplete is active, also reserve space for it
	totalBottomHeight := 1 + inputHeight
	if autocompleteHeight > 0 {
		totalBottomHeight += autocompleteHeight
	}

	contentHeight := h - totalBottomHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	s.msgView.SetDimensions(w, contentHeight)
	s.msgView.SetReservedLines(2)
	s.inputArea.SetWidth(w)
	s.statusBar.SetWidth(w)

	// Set autocomplete view dimensions - use full width
	s.autocompleteView.SetDimensions(w, 0) // Full width, descriptionWidth not used
}

// SetMessages updates the message view.
func (s *Screen) SetMessages(messages []*types.Message) {
	s.msgView.SetMessages(messages)
}

// AddMessage adds a message to the view.
func (s *Screen) AddMessage(msg *types.Message) {
	s.msgView.AddMessage(msg)
}

// SetInputValue sets the input area value.
func (s *Screen) SetInputValue(v string) {
	s.inputArea.SetValue(v)
}

// SetBlocked sets whether the input area is visually blocked.
func (s *Screen) SetBlocked(blocked bool) {
	s.inputArea.SetBlocked(blocked)
}

// SetModel sets the model name in the status bar.
func (s *Screen) SetModel(model string) {
	s.statusBar.SetModel(model)
}

// SetTokenCount sets the token count in the status bar.
func (s *Screen) SetTokenCount(count int) {
	s.statusBar.SetTokenCount(count)
}

// SetMaxTokens sets the max tokens in the status bar.
func (s *Screen) SetMaxTokens(max int) {
	s.statusBar.SetMaxTokens(max)
}

// SetStatusBarState sets the status bar state.
func (s *Screen) SetStatusBarState(state StatusBarState) {
	s.statusBar.SetState(state)
}

// SetToolName sets the name of the tool being executed.
func (s *Screen) SetToolName(name string) {
	s.statusBar.SetToolName(name)
}

// SetStatusMessage sets a custom status message.
func (s *Screen) SetStatusMessage(msg string) {
	s.statusBar.SetStatusMessage(msg)
}

// Tick advances the status bar spinner animation and message view cursor frame.
func (s *Screen) Tick() {
	s.statusBar.Tick()
	s.msgView.cursorFrame++
}

// SetStreaming sets whether the LLM is streaming.
// When streaming is false, only reset to idle if not already in an active state
// (e.g. thinking, tool_call) to avoid clobbering the status during prefill.
func (s *Screen) SetStreaming(streaming bool) {
	s.streaming = streaming
	if streaming {
		s.statusBar.SetState(StatusStreaming)
	} else if !s.statusBar.IsActive() {
		s.statusBar.SetState(StatusIdle)
	}
}

// Render returns the complete screen as a string.
//
// Fixed layout: message view, input area, optional autocomplete panel, status bar.
// The status bar carries the ↓N scroll hint when the message view is scrolled above the bottom.
func (s *Screen) Render() string {
	s.statusBar.SetScrollHint(s.msgView.LinesBelow())

	parts := []string{
		s.msgView.Render(),
		s.inputArea.Render(),
	}

	// Render autocomplete panel if active
	autocompleteState := s.inputArea.GetAutocompleteState()
	if autocompleteState != nil && autocompleteState.IsActive() {
		prefix := autocompleteState.GetPrefix()
		autocompletePanel := s.autocompleteView.Render(autocompleteState, prefix)
		if autocompletePanel != "" {
			parts = append(parts, autocompletePanel)
		}
	}

	parts = append(parts, s.statusBar.Render())
	return strings.Join(parts, "\n")
}

// IsScrolledUp returns true when the message view is scrolled above the bottom.
func (s *Screen) IsScrolledUp() bool {
	return s.msgView.IsScrolledUp()
}

// GetInputArea returns the input area for key handling.
func (s *Screen) GetInputArea() *InputArea {
	return s.inputArea
}

// GetMessageView returns the message view for scroll handling.
func (s *Screen) GetMessageView() *MessageView {
	return s.msgView
}
