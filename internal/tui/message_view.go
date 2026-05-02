package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/user/mmok/internal/types"
)

// messageLineRange maps a message index to its line range in the rendered output.
type messageLineRange struct {
	msgIndex  int
	startLine int
	endLine   int // exclusive
}

// MessageView renders the conversation message list.
type MessageView struct {
	theme       Theme
	messages    []*types.Message
	scrollPos   int
	width       int
	height      int
	visible     int // number of visible lines
	autoScroll  bool
	cursorFrame int                // frame counter for blinking cursor
	lineRanges  []messageLineRange // built during Render
}

// NewMessageView creates a new MessageView.
func NewMessageView(theme Theme) *MessageView {
	return &MessageView{
		theme:      theme,
		autoScroll: true,
	}
}

// SetMessages sets the message list.
func (v *MessageView) SetMessages(messages []*types.Message) {
	v.messages = messages
}

// AddMessage appends a message.
func (v *MessageView) AddMessage(msg *types.Message) {
	v.messages = append(v.messages, msg)
	v.autoScroll = true
}

// SetDimensions sets the viewport dimensions.
func (v *MessageView) SetDimensions(w, h int) {
	v.width = w
	v.height = h
	v.visible = h
}

// ScrollUp moves the scroll position up by one line.
func (v *MessageView) ScrollUp() {
	if v.scrollPos > 0 {
		v.scrollPos--
		v.autoScroll = false
	}
}

// ScrollDown moves the scroll position down by one line.
func (v *MessageView) ScrollDown() {
	totalLines := v.totalLineCount()
	if v.scrollPos < totalLines-v.visible {
		v.scrollPos++
	}
}

// ScrollPageUp moves the scroll position up by one page (viewport height).
func (v *MessageView) ScrollPageUp() {
	if v.scrollPos == 0 {
		return
	}
	v.scrollPos = max(0, v.scrollPos-v.visible+1)
	v.autoScroll = false
}

// ScrollPageDown moves the scroll position down by one page (viewport height).
func (v *MessageView) ScrollPageDown() {
	totalLines := v.totalLineCount()
	maxScroll := max(0, totalLines-v.visible)
	v.scrollPos = min(v.scrollPos+v.visible-1, maxScroll)
}

// ScrollToTop scrolls to the top.
func (v *MessageView) ScrollToTop() {
	v.scrollPos = 0
	v.autoScroll = false
}

// ScrollToBottom scrolls to the bottom.
func (v *MessageView) ScrollToBottom() {
	v.scrollPos = max(0, v.totalLineCount()-v.visible)
	v.autoScroll = true
}

// IsAtBottom returns true when the viewport is showing the last visible lines.
func (v *MessageView) IsAtBottom() bool {
	totalLines := v.totalLineCount()
	return v.scrollPos >= totalLines-v.visible
}

// IsScrolledUp returns true when the viewport is scrolled above the bottom.
func (v *MessageView) IsScrolledUp() bool {
	return !v.IsAtBottom()
}

// totalLineCount returns the total number of rendered lines.
func (v *MessageView) totalLineCount() int {
	total := 0
	for _, msg := range v.messages {
		total += v.messageLineCount(msg)
	}
	return total
}

// messageLineCount returns how many lines a message takes when rendered.
func (v *MessageView) messageLineCount(msg *types.Message) int {
	lines := 0

	// Thinking text: one collapsed line or full content when expanded
	if msg.ThinkingText != "" {
		if msg.ThinkingExpanded {
			wrapped := wordwrap.String("  [thinking] "+msg.ThinkingText, v.width-2)
			lines += len(strings.Split(wrapped, "\n"))
		} else {
			lines++
		}
	}

	switch msg.Type {
	case types.MsgToolCall:
		content := fmt.Sprintf("%s %s", v.messageLabelText(msg), truncate(msg.ToolArgs, 80))
		wrapped := wordwrap.String(content, v.width-2)
		lines += len(strings.Split(wrapped, "\n"))
	case types.MsgToolResult:
		if msg.Collapsed && msg.Summary != "" {
			// Collapsed: just the summary line
			lines = 1
		} else {
			content := msg.Content
			if content != "" {
				text := v.messageLabelText(msg) + " " + content
				wrapped := wordwrap.String(text, v.width-2)
				lines += len(strings.Split(wrapped, "\n"))
			} else {
				lines++
			}
		}
	default:
		content := msg.Content
		if content != "" || msg.ThinkingText != "" {
			text := v.messageLabelText(msg) + " " + content
			wrapped := wordwrap.String(text, v.width-2)
			lines += len(strings.Split(wrapped, "\n"))
		} else {
			lines = 1
		}
		// User messages get 2 extra lines for vertical padding (top + bottom)
		if msg.Type == types.MsgUser {
			lines += 2
		}
	}

	return lines
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// messageLabelText returns the plain text label for a message (no styling).
func (v *MessageView) messageLabelText(msg *types.Message) string {
	switch msg.Type {
	case types.MsgUser:
		return ""
	case types.MsgAssistant:
		return ""
	case types.MsgToolCall:
		return "[" + msg.ToolName + "]"
	case types.MsgToolResult:
		if msg.IsError {
			return "[" + msg.ToolName + "]"
		}
		return "[" + msg.ToolName + "]"
	case types.MsgSystem:
		return "system"
	default:
		return "unknown"
	}
}

// messageStyle returns the appropriate style for a message's content.
func (v *MessageView) messageStyle(msg *types.Message) lipgloss.Style {
	switch msg.Type {
	case types.MsgUser:
		return v.theme.User
	case types.MsgAssistant:
		return v.theme.Assistant
	case types.MsgToolCall:
		return v.theme.ToolCall
	case types.MsgToolResult:
		if msg.IsError {
			return v.theme.Error
		}
		if msg.Collapsed {
			return v.theme.ToolResultCollapsed
		}
		return v.theme.ToolResult
	case types.MsgSystem:
		return v.theme.Dim
	default:
		return lipgloss.NewStyle()
	}
}

// tagStyle returns the style for the tool name tag (e.g. [read]).
func (v *MessageView) tagStyle(msg *types.Message) lipgloss.Style {
	switch msg.Type {
	case types.MsgToolCall:
		return v.theme.ToolCall
	case types.MsgToolResult:
		if msg.IsError {
			return v.theme.Error
		}
		return v.theme.ToolResult
	default:
		return lipgloss.NewStyle()
	}
}

// Render returns the message view as a string.
func (v *MessageView) Render() string {
	if len(v.messages) == 0 {
		centered := v.theme.Dim.Render("No messages yet. Type something!")
		return StringsRepeat("\n", max(0, v.height/2-1)) + centered
	}

	// Build all lines and track line ranges per message
	var allLines []string
	v.lineRanges = make([]messageLineRange, 0, len(v.messages))
	for i, msg := range v.messages {
		startLine := len(allLines)
		lines := v.renderMessage(msg)
		allLines = append(allLines, lines...)
		v.lineRanges = append(v.lineRanges, messageLineRange{
			msgIndex:  i,
			startLine: startLine,
			endLine:   len(allLines),
		})
	}

	// Auto-scroll if needed
	if v.autoScroll {
		v.scrollPos = max(0, len(allLines)-v.visible)
	}

	// Clamp scroll position
	maxScroll := max(0, len(allLines)-v.visible)
	if v.scrollPos > maxScroll {
		v.scrollPos = maxScroll
	}

	// Slice visible lines
	start := v.scrollPos
	end := min(start+v.visible, len(allLines))
	visibleLines := allLines[start:end]

	// Pad to fill height
	for len(visibleLines) < v.visible {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}

// renderMessage renders a single message to wrapped lines.
func (v *MessageView) renderMessage(msg *types.Message) []string {
	style := v.messageStyle(msg)

	var lines []string

	// Render thinking text: collapsed indicator or full text
	if msg.ThinkingText != "" {
		if msg.ThinkingExpanded {
			text := "  [thinking] " + msg.ThinkingText
			wrapped := wordwrap.String(text, v.width-2)
			for _, line := range strings.Split(wrapped, "\n") {
				lines = append(lines, v.theme.Dim.Render(line))
			}
		} else {
			hint := "  [thinking]  (click to expand)"
			lines = append(lines, v.theme.Dim.Render(hint))
		}
	}

	var content string
	switch msg.Type {
	case types.MsgToolCall:
		content = truncate(msg.ToolArgs, 80)
	case types.MsgToolResult:
		if msg.Collapsed && msg.Summary != "" {
			// Show collapsed summary with expand hint
			tag := v.tagStyle(msg).Render("[" + msg.ToolName + "]")
			rest := v.theme.Dim.Render(" " + msg.Summary + "  (click to expand)")
			lines = append(lines, "  "+tag+rest)
			return lines
		}
		content = msg.Content
	default:
		content = msg.Content
	}

	// User messages: top padding line (empty line with background)
	if msg.Type == types.MsgUser {
		style = style.Width(v.width - 2)
		lines = append(lines, style.Render(strings.Repeat(" ", v.width-2)))
	}

	// Use plain label text for wrapping, then apply style uniformly to all lines
	labelText := v.messageLabelText(msg)
	text := labelText + " " + content
	wrapped := wordwrap.String(text, v.width-2)

	// For tool messages, style only the tag, not the full line
	if msg.Type == types.MsgToolCall || msg.Type == types.MsgToolResult {
		tag := v.messageLabelText(msg)
		tagStyler := v.tagStyle(msg)
		var contentStyler lipgloss.Style
		if msg.Type == types.MsgToolCall {
			contentStyler = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
		} else {
			contentStyler = v.theme.Dim
		}

		contentLines := strings.Split(wrapped, "\n")
		for i, line := range contentLines {
			if i == 0 && strings.HasPrefix(line, tag) {
				// First line: style the tag, dim the rest
				rest := line[len(tag):]
				styledLine := tagStyler.Render(tag) + contentStyler.Render(rest)
				lines = append(lines, styledLine)
			} else {
				// Continuation lines: all dim
				lines = append(lines, contentStyler.Render(line))
			}
		}
	} else {
		style = style.Width(v.width - 2)
		contentLines := strings.Split(wrapped, "\n")

		for _, line := range contentLines {
			lines = append(lines, style.Render(line))
		}

	}

	// User messages: bottom padding line
	if msg.Type == types.MsgUser {
		lines = append(lines, style.Render(strings.Repeat(" ", v.width-2)))
	}

	// If streaming, add a blinking cursor indicator
	if msg.Streaming {
		// Blink: 400ms on, 400ms off (4 frames each at 100ms/tick)
		if v.cursorFrame%8 < 4 {
			cursorLine := style.Render("▌")
			lines = append(lines, cursorLine)
		} else {
			lines = append(lines, "")
		}
	}

	return lines
}

// MessageAtY returns the message index at a given screen Y coordinate (0-based),
// or -1 if no message is at that position.
func (v *MessageView) MessageAtY(y int) int {
	// Convert screen Y to absolute line index
	absLine := y + v.scrollPos
	for _, lr := range v.lineRanges {
		if absLine >= lr.startLine && absLine < lr.endLine {
			return lr.msgIndex
		}
	}
	return -1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
