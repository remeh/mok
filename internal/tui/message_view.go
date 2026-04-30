package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/user/mmok/internal/types"
)

// MessageView renders the conversation message list.
type MessageView struct {
	theme      Theme
	messages   []*types.Message
	scrollPos  int
	width      int
	height     int
	visible    int // number of visible lines
	autoScroll bool
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

// ScrollUp moves the scroll position up.
func (v *MessageView) ScrollUp() {
	if v.scrollPos > 0 {
		v.scrollPos--
		v.autoScroll = false
	}
}

// ScrollDown moves the scroll position down.
func (v *MessageView) ScrollDown() {
	totalLines := v.totalLineCount()
	if v.scrollPos < totalLines-v.visible {
		v.scrollPos++
	}
}

// ScrollToBottom scrolls to the bottom.
func (v *MessageView) ScrollToBottom() {
	v.scrollPos = max(0, v.totalLineCount()-v.visible)
	v.autoScroll = true
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
	label := v.messageLabel(msg)
	content := msg.Content

	if msg.Type == types.MsgToolCall {
		content = fmt.Sprintf("[%s] %s(%s)", msg.ToolName, msg.ToolName, msg.ToolArgs)
	}

	lines := 0
	if content != "" || msg.ThinkingText != "" {
		// Thinking text shows as one collapsed line
		thinkingLine := ""
		if msg.ThinkingText != "" {
			thinkingLine = v.theme.Dim.Render("  [thinking]") + "\n"
			lines++
		}
		text := label + " " + content
		wrapped := wordwrap.String(text, v.width-2)
		lines += len(strings.Split(wrapped, "\n"))
		_ = thinkingLine
	} else {
		// Empty streaming message: one line for label
		lines = 1
	}
	return lines
}

// messageLabel returns the styled label prefix for a message.
func (v *MessageView) messageLabel(msg *types.Message) string {
	switch msg.Type {
	case types.MsgUser:
		return v.theme.User.Render("You")
	case types.MsgAssistant:
		return v.theme.Assistant.Render("Assistant")
	case types.MsgToolCall:
		return v.theme.ToolCall.Render("tool_call")
	case types.MsgToolResult:
		if msg.IsError {
			return v.theme.Error.Render("tool_error")
		}
		return v.theme.ToolResult.Render("tool_result")
	case types.MsgSystem:
		return v.theme.Dim.Render("system")
	default:
		return v.theme.Dim.Render("unknown")
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
		return v.theme.ToolResult
	case types.MsgSystem:
		return v.theme.Dim
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

	// Build all lines
	var allLines []string
	for _, msg := range v.messages {
		lines := v.renderMessage(msg)
		allLines = append(allLines, lines...)
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
	label := v.messageLabel(msg)
	style := v.messageStyle(msg)

	var lines []string

	// Render thinking text as a collapsed indicator
	if msg.ThinkingText != "" {
		thinkingIndicator := v.theme.Dim.Render("  [thinking]")
		lines = append(lines, thinkingIndicator)
	}

	var content string
	switch msg.Type {
	case types.MsgToolCall:
		content = fmt.Sprintf("%s(%s)", msg.ToolName, msg.ToolArgs)
	case types.MsgToolResult:
		content = msg.Content
	default:
		content = msg.Content
	}

	text := label + " " + content
	wrapped := wordwrap.String(text, v.width-2)

	contentLines := strings.Split(wrapped, "\n")
	for _, line := range contentLines {
		lines = append(lines, style.Render(line))
	}

	// If streaming, add a cursor indicator
	if msg.Streaming {
		cursorLine := style.Render("▌")
		lines = append(lines, cursorLine)
	}

	return lines
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


