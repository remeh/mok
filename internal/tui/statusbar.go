package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// StatusBarState represents the current application status.
type StatusBarState string

const (
	StatusIdle       StatusBarState = "idle"
	StatusStreaming  StatusBarState = "streaming..."
	StatusCompacting StatusBarState = "compacting..."
	StatusError      StatusBarState = "error"
	StatusThinking   StatusBarState = "thinking..."
)

// StatusBar renders the bottom status bar.
type StatusBar struct {
	theme          Theme
	model          string
	tokenCount     int
	maxTokens      int
	state          StatusBarState
	width          int
}

// NewStatusBar creates a new StatusBar.
func NewStatusBar(theme Theme) *StatusBar {
	return &StatusBar{
		theme:     theme,
		state:     StatusIdle,
		maxTokens: 131072,
	}
}

// SetModel sets the model name.
func (s *StatusBar) SetModel(model string) {
	s.model = model
}

// SetTokenCount sets the current token count.
func (s *StatusBar) SetTokenCount(count int) {
	s.tokenCount = count
}

// SetMaxTokens sets the max context tokens.
func (s *StatusBar) SetMaxTokens(max int) {
	if max > 0 {
		s.maxTokens = max
	}
}

// SetState sets the status state.
func (s *StatusBar) SetState(state StatusBarState) {
	s.state = state
}

// SetWidth sets the bar width.
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// Render returns the styled status bar string.
func (s *StatusBar) Render() string {
	if s.width == 0 {
		s.width = 80
	}

	// Left: model name
	left := s.theme.StatusBar.Render(s.model)

	// Center: token count / context usage
	var tokenInfo string
	if s.maxTokens > 0 {
		pct := float64(s.tokenCount) / float64(s.maxTokens) * 100
		tokenInfo = fmt.Sprintf("%d/%d tokens (%.0f%%)", s.tokenCount, s.maxTokens, pct)
	} else {
		tokenInfo = fmt.Sprintf("%d tokens", s.tokenCount)
	}
	center := s.theme.StatusBar.Render(tokenInfo)

	// Right: status
	right := s.theme.StatusBar.Render(string(s.state))

	// Combine with spacing
	totalLen := lipgloss.Width(left) + lipgloss.Width(center) + lipgloss.Width(right) + 4
	padding := s.width - totalLen
	if padding < 0 {
		padding = 0
	}

	leftPad := padding / 2
	rightPad := padding - leftPad

	return fmt.Sprintf("%s%s %s %s%s",
		s.theme.StatusBar.Render(StringsRepeat(" ", leftPad)),
		left,
		center,
		right,
		s.theme.StatusBar.Render(StringsRepeat(" ", rightPad)))
}


