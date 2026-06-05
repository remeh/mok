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
	StatusProcessing StatusBarState = "processing"
	StatusToolCall   StatusBarState = "tool_call"
	StatusWaitingConfirm StatusBarState = "waiting_confirm"
	StatusYolo       StatusBarState = "yolo"
)

// StatusBar renders the bottom status bar.
type StatusBar struct {
	theme         Theme
	model         string
	tokenCount    int
	maxTokens     int
	state         StatusBarState
	toolName      string // Name of the tool being executed
	statusMessage string // Custom status message
	scrollHint    int    // lines below the viewport; rendered as ↓N when > 0
	width         int
	dotPhase      int // 0..2, cycles to produce ".  " ".. " "..."
	tickCount     int // raw tick counter; dotPhase advances every dotTickInterval ticks
	lastUpdate    time.Time
	yoloMode      bool // when true, show YOLO indicator
}

const dotTickInterval = 10 // advance dots every 10 ticks (~1s at 100ms tick rate)

// NewStatusBar creates a new StatusBar.
func NewStatusBar(theme Theme) *StatusBar {
	return &StatusBar{
		theme:      theme,
		state:      StatusIdle,
		maxTokens:  131072,
		dotPhase:   2, // start at 3 dots so initial render matches "..."
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

// TokenCount returns the current token count.
func (s *StatusBar) TokenCount() int {
	return s.tokenCount
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

// SetStatusMessage sets a custom status message.
func (s *StatusBar) SetStatusMessage(msg string) {
	s.statusMessage = msg
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

// SetYoloMode sets the YOLO mode indicator.
func (s *StatusBar) SetYoloMode(enabled bool) {
	s.yoloMode = enabled
}

// Tick advances the dot animation.
func (s *StatusBar) Tick() {
	s.tickCount++
	if s.tickCount%dotTickInterval == 0 {
		s.dotPhase = (s.dotPhase + 1) % 3
	}
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

	// Right: status with dot animation when active
	right := s.renderStatus()

	// Calculate padding for centering
	totalLen := lipgloss.Width(left) + lipgloss.Width(middle) + lipgloss.Width(right) + 2
	padding := s.width - totalLen
	if padding < 0 {
		padding = 0
	}

	leftPad := padding / 2
	rightPad := padding - leftPad

	// Combine with spacing - ensure spaces between segments have the same background
	spacer := s.theme.StatusBar.Render(" ")
	return fmt.Sprintf("%s%s%s%s%s%s%s",
		s.theme.StatusBar.Render(StringsRepeat(" ", leftPad)),
		left,
		spacer,
		middle,
		spacer,
		right,
		s.theme.StatusBar.Render(StringsRepeat(" ", rightPad)))
}

func (s *StatusBar) renderStatus() string {
	// If there's a custom status message, show it
	if s.statusMessage != "" {
		return s.theme.StatusBarActive.Render(s.statusMessage)
	}

	// Build the base status string
	var baseStatus string
	switch s.state {
	case StatusIdle:
		baseStatus = s.theme.StatusBarIdle.Render("● ready")
	case StatusError:
		baseStatus = s.theme.StatusBarError.Render("✗ error")
	case StatusToolCall:
		dots := dotSuffix(s.dotPhase)
		if s.toolName != "" {
			baseStatus = s.theme.StatusBarActive.Render("executing: " + s.toolName + dots)
		} else {
			baseStatus = s.theme.StatusBarActive.Render("executing tool" + dots)
		}
	case StatusProcessing:
		dots := dotSuffix(s.dotPhase)
		baseStatus = s.theme.StatusBarActive.Render("processing" + dots)
	case StatusStreaming:
		dots := dotSuffix(s.dotPhase)
		baseStatus = s.theme.StatusBarActive.Render("streaming" + dots)
	case StatusCompacting:
		dots := dotSuffix(s.dotPhase)
		baseStatus = s.theme.StatusBarActive.Render("compacting" + dots)
	case StatusWaitingConfirm:
		baseStatus = s.theme.StatusBarActive.Render("⧖ waiting for confirmation...")
	case StatusYolo:
		// Fallback for backward compatibility - should use SetYoloMode instead
		baseStatus = s.theme.StatusBarError.Render("🟥 YOLO")
	default:
		baseStatus = s.theme.StatusBar.Render(string(s.state))
	}

	// Append YOLO indicator if enabled
	if s.yoloMode {
		yoloIndicator := s.theme.StatusBarError.Render(" 🟥 YOLO")
		return baseStatus + yoloIndicator
	}

	return baseStatus
}

// dotSuffix returns a fixed-width (3 chars) dot suffix: ".  " ".. " "..."
func dotSuffix(phase int) string {
	switch phase {
	case 0:
		return ".  "
	case 1:
		return ".. "
	default:
		return "..."
	}
}

// IsActive returns true if the status bar should show an active state.
func (s *StatusBar) IsActive() bool {
	return s.state != StatusIdle && s.state != StatusError
}
