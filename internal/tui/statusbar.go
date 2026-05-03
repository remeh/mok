package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// StatusBarState represents the current application status.
type StatusBarState string

const (
	StatusIdle       StatusBarState = "idle"
	StatusStreaming  StatusBarState = "streaming"
	StatusCompacting StatusBarState = "compacting"
	StatusError      StatusBarState = "error"
	StatusThinking   StatusBarState = "thinking"
	StatusToolCall   StatusBarState = "tool_call"
)

// StatusBar renders the bottom status bar.
type StatusBar struct {
	theme        Theme
	model        string
	tokenCount   int
	maxTokens    int
	state        StatusBarState
	toolName     string // Name of the tool being executed
	scrollHint   int    // lines below the viewport; rendered as ↓N when > 0
	width        int
	spinnerFrame int
	lastUpdate   time.Time
}

// Spinner frames for activity indicator
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewStatusBar creates a new StatusBar.
func NewStatusBar(theme Theme) *StatusBar {
	return &StatusBar{
		theme:      theme,
		state:      StatusIdle,
		maxTokens:  131072,
		lastUpdate: time.Now(),
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
	s.lastUpdate = time.Now()
}

// SetToolName sets the name of the tool being executed.
func (s *StatusBar) SetToolName(name string) {
	s.toolName = name
}

// SetScrollHint sets the count of lines below the viewport. Non-zero values
// render as a ↓N segment so the user knows there's content out of view.
func (s *StatusBar) SetScrollHint(linesBelow int) {
	if linesBelow < 0 {
		linesBelow = 0
	}
	s.scrollHint = linesBelow
}

// SetWidth sets the bar width.
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// Tick advances the spinner animation. Call periodically (e.g., from a tea.Tick cmd).
func (s *StatusBar) Tick() {
	s.spinnerFrame = (s.spinnerFrame + 1) % len(spinnerFrames)
}

// Render returns the styled status bar string.
func (s *StatusBar) Render() string {
	if s.width == 0 {
		s.width = 80
	}

	// Left: model name
	left := s.theme.StatusBar.Render(s.model)

	// Center: token count / context usage, optionally followed by ↓N hint
	// when there is content scrolled below the viewport.
	var tokenInfo string
	if s.maxTokens > 0 {
		pct := float64(s.tokenCount) / float64(s.maxTokens) * 100
		tokenInfo = fmt.Sprintf("%d/%d tokens (%.0f%%)", s.tokenCount, s.maxTokens, pct)
	} else {
		tokenInfo = fmt.Sprintf("%d tokens", s.tokenCount)
	}
	middle := s.theme.StatusBar.Render(tokenInfo)
	if s.scrollHint > 0 {
		hint := s.theme.StatusBarActive.Render(fmt.Sprintf("↓%d", s.scrollHint))
		middle = middle + " " + hint
	}

	// Right: status with spinner when active
	right := s.renderStatus()

	// Combine with spacing
	totalLen := lipgloss.Width(left) + lipgloss.Width(middle) + lipgloss.Width(right) + 4
	padding := s.width - totalLen
	if padding < 0 {
		padding = 0
	}

	leftPad := padding / 2
	rightPad := padding - leftPad

	return fmt.Sprintf("%s%s %s %s%s",
		s.theme.StatusBar.Render(StringsRepeat(" ", leftPad)),
		left,
		middle,
		right,
		s.theme.StatusBar.Render(StringsRepeat(" ", rightPad)))
}

func (s *StatusBar) renderStatus() string {
	switch s.state {
	case StatusIdle:
		return s.theme.StatusBarIdle.Render("● ready")
	case StatusError:
		return s.theme.StatusBarError.Render("✗ error")
	case StatusToolCall:
		if s.toolName != "" {
			return s.theme.StatusBarActive.Render(spinnerFrames[s.spinnerFrame] + " executing: " + s.toolName)
		}
		return s.theme.StatusBarActive.Render(spinnerFrames[s.spinnerFrame] + " executing tool...")
	case StatusThinking:
		return s.theme.StatusBarActive.Render(spinnerFrames[s.spinnerFrame] + " thinking...")
	case StatusStreaming:
		return s.theme.StatusBarActive.Render(spinnerFrames[s.spinnerFrame] + " streaming...")
	case StatusCompacting:
		return s.theme.StatusBarActive.Render(spinnerFrames[s.spinnerFrame] + " compacting...")
	default:
		return s.theme.StatusBar.Render(string(s.state))
	}
}

// IsActive returns true if the status bar should show an active state.
func (s *StatusBar) IsActive() bool {
	return s.state != StatusIdle && s.state != StatusError
}
