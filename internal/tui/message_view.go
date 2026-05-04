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

// renderedMessage is a cached, fully-rendered form of a single message.
//
// The cache is the single source of truth for line counts: scroll math
// (IsAtBottom, ScrollPage*, clamping) reads len(lines) from here so the
// estimated line count and the actual rendered output can never disagree.
type renderedMessage struct {
	lines       []string
	fingerprint string
}

// MessageView renders the conversation message list.
//
// Scroll state machine (Phase 2):
//   - pinned == false: viewport follows the tail of the message list. Any
//     content growth (new message, streaming delta) snaps scrollPos to bottom.
//   - pinned == true:  user has taken explicit control. scrollPos is never
//     moved by content arrival.
//
// pinned is set by any explicit scroll-up gesture and cleared by any gesture
// that lands at the true bottom (ScrollDown to maxScroll, ScrollToBottom,
// new prompt submission). Render() never mutates pinned or scrollPos.
type MessageView struct {
	theme         Theme
	messages      []*types.Message
	rendered      []renderedMessage // parallel to messages; rebuilt lazily
	scrollPos     int
	width         int
	height        int
	visible       int // number of visible lines
	pinned        bool
	cursorFrame   int                // frame counter for blinking cursor
	lineRanges    []messageLineRange // built during Render
	mdRenderer    *markdownRenderer  // lazily initialized markdown renderer
	reservedLines int                // lines reserved below the message view (input + status bar)
}

// NewMessageView creates a new MessageView.
func NewMessageView(theme Theme) *MessageView {
	// pinned defaults to false: follow tail until the user scrolls up.
	return &MessageView{theme: theme}
}

// SetMessages sets the message list.
func (v *MessageView) SetMessages(messages []*types.Message) {
	v.messages = messages
}

// AddMessage appends a message and follows the tail if not pinned.
func (v *MessageView) AddMessage(msg *types.Message) {
	v.messages = append(v.messages, msg)
	v.maybeFollowTail()
}

// MessageGrew should be called after the content of an existing message has
// changed in place (typically a streaming text or thinking delta). Snaps the
// viewport to the new bottom unless the user has pinned the scroll.
func (v *MessageView) MessageGrew() {
	v.maybeFollowTail()
}

// Clear removes all messages and resets the view.
func (v *MessageView) Clear() {
	v.messages = nil
	v.rendered = nil
	v.scrollPos = 0
	v.pinned = false
}

// maybeFollowTail snaps scrollPos to the bottom of the rendered content,
// unless the user has explicitly pinned the scroll position.
func (v *MessageView) maybeFollowTail() {
	if v.pinned {
		return
	}
	// Until SetDimensions has been called we can't render or measure
	// anything. The next Render() will be preceded by a SetDimensions and
	// followed by an explicit ScrollToBottom on submit (or the default
	// scrollPos of 0, which is correct for an empty view).
	if v.width <= 2 || v.visible <= 0 {
		return
	}
	v.scrollPos = max(0, v.totalLineCount()-v.visible)
}

// SetReservedLines sets the number of lines reserved below the message view
// (typically input line + status bar = 2).
func (v *MessageView) SetReservedLines(n int) {
	v.reservedLines = n
}

// SetDimensions sets the viewport dimensions.
func (v *MessageView) SetDimensions(w, h int) {
	if v.width != w {
		// Width change invalidates the markdown renderer (its wordwrap is
		// width-bound). The render cache is invalidated automatically because
		// every fingerprint embeds v.width.
		v.mdRenderer = nil
	}
	v.width = w
	v.height = h
	v.visible = h
}

// ScrollUp moves the scroll position up by one line. Pins the scroll.
func (v *MessageView) ScrollUp() {
	if v.scrollPos > 0 {
		v.scrollPos--
		v.pinned = true
	}
}

// ScrollDown moves the scroll position down by one line. Unpins if it lands
// at the true bottom.
func (v *MessageView) ScrollDown() {
	maxScroll := max(0, v.totalLineCount()-v.visible)
	if v.scrollPos < maxScroll {
		v.scrollPos++
	}
	if v.scrollPos >= maxScroll {
		v.pinned = false
	}
}

// ScrollPageUp moves the scroll position up by one page. Pins the scroll.
func (v *MessageView) ScrollPageUp() {
	if v.scrollPos == 0 {
		return
	}
	v.scrollPos = max(0, v.scrollPos-v.visible+1)
	v.pinned = true
}

// ScrollPageDown moves the scroll position down by one page. Unpins if it
// lands at the true bottom.
func (v *MessageView) ScrollPageDown() {
	maxScroll := max(0, v.totalLineCount()-v.visible)
	v.scrollPos = min(v.scrollPos+v.visible-1, maxScroll)
	if v.scrollPos >= maxScroll {
		v.pinned = false
	}
}

// ScrollToTop scrolls to the top and pins.
func (v *MessageView) ScrollToTop() {
	v.scrollPos = 0
	v.pinned = true
}

// ScrollToBottom unpins and snaps to the bottom.
func (v *MessageView) ScrollToBottom() {
	v.pinned = false
	v.scrollPos = max(0, v.totalLineCount()-v.visible)
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

// LinesBelow returns the number of rendered lines below the current viewport.
// Used by Screen to render a ↓N scroll hint in the status bar.
func (v *MessageView) LinesBelow() int {
	below := v.totalLineCount() - v.scrollPos - v.visible
	if below < 0 {
		return 0
	}
	return below
}

// totalLineCount returns the total number of rendered lines, derived from the
// render cache. This is the single source of truth for scroll math.
func (v *MessageView) totalLineCount() int {
	v.ensureRendered()
	total := 0
	for _, r := range v.rendered {
		total += len(r.lines)
	}
	return total
}

// computeFingerprint returns a string that uniquely identifies the visible
// state of a message at the current width. When this string is unchanged, the
// cached rendered lines can be reused.
func (v *MessageView) computeFingerprint(msg *types.Message) string {
	return fmt.Sprintf("%s|%d|%t|%t|%t|%t|%s|%s|%s|%s|%s",
		msg.Type, v.width,
		msg.ThinkingExpanded, msg.Streaming, msg.Collapsed, msg.IsError,
		msg.ToolName, msg.ToolArgs, msg.Summary,
		msg.ThinkingText, msg.Content,
	)
}

// ensureRendered makes sure v.rendered is in sync with v.messages at v.width.
// Streaming messages are always re-rendered (their content is mutating in
// place each tick and the cursor blinks), but other messages are only
// re-rendered when their fingerprint changes.
//
// No-op if the view has no usable width yet (the constructor leaves it 0;
// SetDimensions sets it before the first Render).
func (v *MessageView) ensureRendered() {
	if v.width <= 2 {
		return
	}
	if len(v.rendered) > len(v.messages) {
		v.rendered = v.rendered[:len(v.messages)]
	}
	for i, msg := range v.messages {
		if i >= len(v.rendered) {
			v.rendered = append(v.rendered, renderedMessage{})
		}
		fp := v.computeFingerprint(msg)
		if msg.Streaming || v.rendered[i].fingerprint != fp {
			v.rendered[i] = renderedMessage{
				lines:       v.renderMessageLines(msg),
				fingerprint: fp,
			}
		}
	}
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
	case types.MsgSystem:
		return "[system]"
	case types.MsgAssistant:
		return ""
	case types.MsgToolCall:
		return "[" + msg.ToolName + "]"
	case types.MsgToolResult:
		if msg.IsError {
			return "[" + msg.ToolName + "]"
		}
		return "[" + msg.ToolName + "]"
	default:
		return "unknown"
	}
}

// messageStyle returns the appropriate style for a message's content.
func (v *MessageView) messageStyle(msg *types.Message) lipgloss.Style {
	switch msg.Type {
	case types.MsgUser:
		return v.theme.User
	case types.MsgSystem:
		return v.theme.System
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
		fullHeight := v.height + v.reservedLines
		centered := v.theme.Dim.Copy().
			AlignHorizontal(lipgloss.Center).
			Width(v.width).
			Render("No messages yet. Start coding now!")
		centerLine := fullHeight/2 - 1
		topPadding := StringsRepeat("\n", max(0, centerLine))
		bottomPadding := StringsRepeat("\n", max(0, v.height-centerLine-1))
		return topPadding + centered + bottomPadding
	}

	v.ensureRendered()

	// Flatten the cache and track line ranges per message.
	var allLines []string
	v.lineRanges = make([]messageLineRange, 0, len(v.messages))
	for i := range v.messages {
		startLine := len(allLines)
		allLines = append(allLines, v.rendered[i].lines...)
		v.lineRanges = append(v.lineRanges, messageLineRange{
			msgIndex:  i,
			startLine: startLine,
			endLine:   len(allLines),
		})
	}

	// Render() is pure: it does not mutate scroll state. Snapping to the
	// bottom on content arrival is handled by AddMessage / MessageGrew /
	// ScrollToBottom via maybeFollowTail(). We still defensively clamp here
	// because totalLineCount can shrink (e.g. SetMessages with fewer entries)
	// between an explicit scroll call and the next Render.
	maxScroll := max(0, len(allLines)-v.visible)
	if v.scrollPos > maxScroll {
		v.scrollPos = maxScroll
	}

	// Slice visible lines.
	start := v.scrollPos
	end := min(start+v.visible, len(allLines))
	visibleLines := allLines[start:end]

	// Pad to fill height.
	for len(visibleLines) < v.visible {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}

// renderMessageLines renders a single message into its final wrapped, styled
// lines. This is the only place rendering happens; messageLineCount no longer
// exists because counts are derived from len(rendered.lines) instead.
func (v *MessageView) renderMessageLines(msg *types.Message) []string {
	style := v.messageStyle(msg)

	var lines []string

	// Render thinking text: collapsed indicator or full text.
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
			// Show collapsed summary with expand hint.
			tag := v.tagStyle(msg).Render("[" + msg.ToolName + "]")
			rest := v.theme.Dim.Render(" " + msg.Summary + "  (click to expand)")
			lines = append(lines, "  "+tag+rest)
			return lines
		}
		content = msg.Content
	default:
		content = msg.Content
	}

	// Markdown rendering for assistant messages (not during streaming).
	if msg.Type == types.MsgAssistant && content != "" && !msg.Streaming {
		if v.mdRenderer == nil {
			if r, err := newMarkdownRenderer(v.width); err == nil {
				v.mdRenderer = r
			}
		}
		if v.mdRenderer != nil {
			if styled, err := v.mdRenderer.Render(content); err == nil {
				content = styled
			}
			// On error, fall through to plain rendering.
		}
	}

	// User messages: top padding line (empty line with background).
	if msg.Type == types.MsgUser {
		paddedStyle := style.Width(v.width - 2)
		lines = append(lines, paddedStyle.Render(strings.Repeat(" ", v.width-2)))
	}

	// Use plain label text for wrapping, then apply style uniformly.
	labelText := v.messageLabelText(msg)
	text := labelText + " " + content
	wrapped := wordwrap.String(text, v.width-2)
	contentLines := strings.Split(wrapped, "\n")

	// Inline streaming cursor: appended to the last content line so the
	// total line count stays constant across blink frames.
	if msg.Streaming && v.cursorFrame%8 < 4 && len(contentLines) > 0 {
		contentLines[len(contentLines)-1] += "▌"
	}

	// For tool messages, style only the tag, not the full line.
	if msg.Type == types.MsgToolCall || msg.Type == types.MsgToolResult {
		tag := v.messageLabelText(msg)
		tagStyler := v.tagStyle(msg)
		var contentStyler lipgloss.Style
		if msg.Type == types.MsgToolCall {
			contentStyler = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
		} else {
			contentStyler = v.theme.Dim
		}

		for i, line := range contentLines {
			if i == 0 && strings.HasPrefix(line, tag) {
				rest := line[len(tag):]
				styledLine := tagStyler.Render(tag) + contentStyler.Render(rest)
				lines = append(lines, styledLine)
			} else {
				lines = append(lines, contentStyler.Render(line))
			}
		}
	} else {
		paddedStyle := style.Width(v.width - 2)
		for _, line := range contentLines {
			lines = append(lines, paddedStyle.Render(line))
		}
	}

	// User messages: bottom padding line.
	if msg.Type == types.MsgUser {
		paddedStyle := style.Width(v.width - 2)
		lines = append(lines, paddedStyle.Render(strings.Repeat(" ", v.width-2)))
	}

	return lines
}

// MessageAtY returns the message index at a given screen Y coordinate (0-based),
// or -1 if no message is at that position.
func (v *MessageView) MessageAtY(y int) int {
	// Convert screen Y to absolute line index.
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
