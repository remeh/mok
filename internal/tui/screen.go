package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/mmok/internal/types"
)

// Screen composes all TUI components into the main layout.
type Screen struct {
	theme     Theme
	msgView   *MessageView
	inputArea *InputArea
	statusBar *StatusBar
	width     int
	height    int
	streaming bool
}

// NewScreen creates a new Screen with all sub-components.
func NewScreen(theme Theme) *Screen {
	return &Screen{
		theme:     theme,
		msgView:   NewMessageView(theme),
		inputArea: NewInputArea(theme, ">"),
		statusBar: NewStatusBar(theme),
	}
}

// SetDimensions updates all component dimensions.
func (s *Screen) SetDimensions(w, h int) {
	s.width = w
	s.height = h

	contentHeight := h - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	s.msgView.SetDimensions(w, contentHeight)
	s.inputArea.SetWidth(w)
	s.statusBar.SetWidth(w)
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
func (s *Screen) Render() string {
	msgLines := s.msgView.Render()

	var inputLine string
	if s.msgView.IsScrolledUp() {
		inputLine = s.RenderScrollIndicator()
	} else {
		inputLine = s.inputArea.Render()
	}

	statusLine := s.statusBar.Render()

	parts := []string{msgLines, inputLine, statusLine}
	return strings.Join(parts, "\n")
}

// RenderScrollIndicator returns a line showing "vvvvvvv" on a white background.
func (s *Screen) RenderScrollIndicator() string {
	indicator := s.theme.ScrollIndicator.Render("vvvvvvv")
	renderedWidth := lipgloss.Width(indicator)
	if renderedWidth < s.width {
		indicator += StringsRepeat(" ", s.width-renderedWidth)
	}
	return indicator
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
