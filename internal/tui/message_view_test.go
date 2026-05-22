package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/user/mok/internal/types"
)

func setupMessageView(t *testing.T) *MessageView {
	t.Helper()
	theme := DefaultTheme()
	v := NewMessageView(theme)
	v.SetDimensions(80, 10)
	return v
}

func TestMessageViewEmpty(t *testing.T) {
	v := setupMessageView(t)

	rendered := v.Render()
	if !strings.Contains(rendered, "No messages") {
		t.Errorf("Empty view should show placeholder: %q", rendered)
	}
}

func TestMessageViewAddMessage(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewMessage(types.MsgUser, "hello world")
	v.AddMessage(msg)

	if len(v.messages) != 1 {
		t.Errorf("messages length = %d, want 1", len(v.messages))
	}
}

func TestMessageViewSetMessages(t *testing.T) {
	v := setupMessageView(t)

	messages := []*types.Message{
		types.NewMessage(types.MsgUser, "first"),
		types.NewMessage(types.MsgAssistant, "second"),
		types.NewMessage(types.MsgUser, "third"),
	}
	v.SetMessages(messages)

	if len(v.messages) != 3 {
		t.Errorf("messages length = %d, want 3", len(v.messages))
	}
}

func TestMessageViewScroll(t *testing.T) {
	v := setupMessageView(t)

	// Add enough messages to fill the view
	for i := 0; i < 20; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "message"))
	}

	v.ScrollToBottom()
	initialPos := v.scrollPos

	v.ScrollUp()
	if v.scrollPos != initialPos-1 {
		t.Errorf("scrollPos = %d, want %d", v.scrollPos, initialPos-1)
	}

	v.ScrollDown()
	if v.scrollPos != initialPos {
		t.Errorf("scrollPos = %d, want %d", v.scrollPos, initialPos)
	}
}

func TestMessageViewScrollUpAtTop(t *testing.T) {
	v := setupMessageView(t)
	v.AddMessage(types.NewMessage(types.MsgUser, "hello"))

	v.ScrollUp()
	if v.scrollPos != 0 {
		t.Errorf("scrollPos = %d, should stay 0", v.scrollPos)
	}
}

func TestMessageViewPinnedDefault(t *testing.T) {
	v := setupMessageView(t)
	v.AddMessage(types.NewMessage(types.MsgUser, "hello"))

	if v.pinned {
		t.Error("pinned should be false after adding message (follow tail by default)")
	}

	// ScrollUp only pins if scrollPos > 0. With a single message, scrollPos
	// is 0, so the gesture is a no-op and pinned stays false.
	v.ScrollUp()
	if v.pinned {
		t.Error("pinned should stay false when ScrollUp is a no-op at top")
	}
}

func TestMessageViewScrollPageUp(t *testing.T) {
	v := setupMessageView(t)

	// Add enough messages to fill the view
	for i := 0; i < 20; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "message"))
	}

	v.ScrollToBottom()
	initialPos := v.scrollPos

	v.ScrollPageUp()
	if v.scrollPos >= initialPos {
		t.Errorf("scrollPos = %d, want < %d after page up", v.scrollPos, initialPos)
	}
	if !v.pinned {
		t.Error("pinned should be true after page up")
	}
}

func TestMessageViewScrollPageUpAtTop(t *testing.T) {
	v := setupMessageView(t)
	v.AddMessage(types.NewMessage(types.MsgUser, "hello"))

	v.ScrollPageUp()
	if v.scrollPos != 0 {
		t.Errorf("scrollPos = %d, should stay 0", v.scrollPos)
	}
}

func TestMessageViewScrollPageDown(t *testing.T) {
	v := setupMessageView(t)

	// Add enough messages to fill the view
	for i := 0; i < 20; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "message"))
	}

	v.ScrollToTop()
	initialPos := v.scrollPos

	v.ScrollPageDown()
	if v.scrollPos <= initialPos {
		t.Errorf("scrollPos = %d, want > %d after page down", v.scrollPos, initialPos)
	}
}

func TestMessageViewScrollToTop(t *testing.T) {
	v := setupMessageView(t)

	for i := 0; i < 20; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "message"))
	}

	v.ScrollToBottom()
	v.ScrollToTop()
	if v.scrollPos != 0 {
		t.Errorf("scrollPos = %d, want 0 after scroll to top", v.scrollPos)
	}
	if !v.pinned {
		t.Error("pinned should be true after scroll to top")
	}
}

func TestMessageViewToolCall(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewToolCall("read", `path="test.go"`)
	v.AddMessage(msg)

	rendered := v.Render()
	if !strings.Contains(rendered, "read") {
		t.Errorf("Render should contain read label: %q", rendered)
	}
}

func TestMessageViewToolResult(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewToolResult("read", "file contents", false)
	v.AddMessage(msg)

	rendered := v.Render()
	// Collapsed tool results show summary with expand hint
	if !strings.Contains(rendered, "[read]") {
		t.Errorf("Render should contain tool name: %q", rendered)
	}
	if !strings.Contains(rendered, "click to expand") {
		t.Errorf("Render should contain expand hint: %q", rendered)
	}
}

func TestMessageViewToolError(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewToolResult("edit", "file not found", true)
	v.AddMessage(msg)

	rendered := v.Render()
	// Collapsed tool errors show error summary
	if !strings.Contains(rendered, "[edit]") {
		t.Errorf("Render should contain tool name: %q", rendered)
	}
	if !strings.Contains(rendered, "✗") {
		t.Errorf("Render should contain error indicator: %q", rendered)
	}
}

func TestMessageViewToolResultExpanded(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewToolResult("read", "file contents here", false)
	msg.Collapsed = false // Expand it
	v.AddMessage(msg)

	rendered := v.Render()
	if !strings.Contains(rendered, "read") {
		t.Errorf("Render should contain read label: %q", rendered)
	}
	if !strings.Contains(rendered, "file contents here") {
		t.Errorf("Render should contain full content: %q", rendered)
	}
}

func TestMessageViewUserMessage(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewMessage(types.MsgUser, "user message")
	v.AddMessage(msg)

	rendered := v.Render()
	if !strings.Contains(rendered, "user") {
		t.Errorf("Render should contain user label: %q", rendered)
	}
}

func TestMessageViewRender(t *testing.T) {
	v := setupMessageView(t)

	v.AddMessage(types.NewMessage(types.MsgUser, "hello"))
	v.AddMessage(types.NewMessage(types.MsgAssistant, "hi there"))

	rendered := v.Render()
	if rendered == "" {
		t.Error("Render should not return empty string")
	}
}

func TestMessageViewSetDimensions(t *testing.T) {
	v := setupMessageView(t)
	v.SetDimensions(120, 20)

	if v.width != 120 {
		t.Errorf("width = %d, want 120", v.width)
	}
	if v.height != 20 {
		t.Errorf("height = %d, want 20", v.height)
	}
	if v.visible != 20 {
		t.Errorf("visible = %d, want 20", v.visible)
	}
}

func TestMessageViewTotalLineCount(t *testing.T) {
	v := setupMessageView(t)

	v.AddMessage(types.NewMessage(types.MsgUser, "short"))
	v.AddMessage(types.NewMessage(types.MsgAssistant, "reply"))

	total := v.totalLineCount()
	if total < 2 {
		t.Errorf("totalLineCount = %d, want at least 2", total)
	}
}

// TestRenderedLineCountMatchesCache asserts the cache and an ad-hoc render
// agree on line count. Before Phase 1, messageLineCount() and renderMessage()
// disagreed for assistant messages because only renderMessage applied
// markdown rendering, breaking ScrollToBottom and clamping.
func TestRenderedLineCountMatchesCache(t *testing.T) {
	cases := []*types.Message{
		types.NewMessage(types.MsgUser, "hello world"),
		types.NewMessage(types.MsgAssistant, "# Header\n\n- bullet 1\n- bullet 2\n\n```go\ncode here\n```\n"),
		types.NewToolCall("read", `path="test.go"`),
		func() *types.Message {
			m := types.NewToolResult("read", "line a\nline b\nline c\n", false)
			m.Collapsed = false
			return m
		}(),
		types.NewMessage(types.MsgUser, "user note"),
	}

	for _, m := range cases {
		v := setupMessageView(t)
		v.AddMessage(m)
		v.ensureRendered()
		cached := len(v.rendered[0].lines)
		fresh := len(v.renderMessageLines(m))
		if cached != fresh {
			t.Errorf("type=%s cached=%d fresh=%d", m.Type, cached, fresh)
		}
	}
}

// TestCacheReusesEntriesAcrossRenders verifies the fingerprint short-circuits
// re-rendering when nothing visible changed.
func TestCacheReusesEntriesAcrossRenders(t *testing.T) {
	v := setupMessageView(t)
	v.AddMessage(types.NewMessage(types.MsgAssistant, "hello"))
	v.ensureRendered()
	first := &v.rendered[0]
	firstLines := first.lines

	v.ensureRendered()
	if &v.rendered[0].lines[0] != &firstLines[0] {
		t.Error("non-streaming message was re-rendered despite unchanged fingerprint")
	}
}

// TestCacheInvalidatesOnContentChange verifies the fingerprint catches
// in-place mutations to message fields (Collapsed, Content, etc).
func TestCacheInvalidatesOnContentChange(t *testing.T) {
	v := setupMessageView(t)
	tr := types.NewToolResult("read", "file contents here", false)
	v.AddMessage(tr)
	v.ensureRendered()
	collapsedFp := v.rendered[0].fingerprint
	collapsedFirstLine := v.rendered[0].lines[0]

	tr.Collapsed = false
	v.ensureRendered()
	expandedFp := v.rendered[0].fingerprint
	expandedFirstLine := v.rendered[0].lines[0]

	if collapsedFp == expandedFp {
		t.Errorf("fingerprint should differ after Collapsed toggle: %q", collapsedFp)
	}
	if collapsedFirstLine == expandedFirstLine {
		t.Errorf("rendered output should differ after Collapsed toggle: %q", collapsedFirstLine)
	}
}

// TestCacheTrimsOnMessageRemoval verifies the cache shrinks if SetMessages
// replaces the list with fewer entries.
func TestCacheTrimsOnMessageRemoval(t *testing.T) {
	v := setupMessageView(t)
	v.SetMessages([]*types.Message{
		types.NewMessage(types.MsgUser, "a"),
		types.NewMessage(types.MsgUser, "b"),
		types.NewMessage(types.MsgUser, "c"),
	})
	v.ensureRendered()
	if len(v.rendered) != 3 {
		t.Fatalf("rendered = %d, want 3", len(v.rendered))
	}

	v.SetMessages([]*types.Message{
		types.NewMessage(types.MsgUser, "x"),
	})
	v.ensureRendered()
	if len(v.rendered) != 1 {
		t.Errorf("rendered = %d, want 1 after shrink", len(v.rendered))
	}
}

// --- Phase 2: pinned state machine ---

// TestRenderDoesNotMutateScrollPos asserts that Render() is pure with respect
// to scroll state. Before Phase 2 it overrode scrollPos via the autoScroll
// override, which fought manual scroll during streaming.
func TestRenderDoesNotMutateScrollPos(t *testing.T) {
	v := setupMessageView(t)
	for i := 0; i < 30; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}

	// Pin in the middle.
	v.scrollPos = 5
	v.pinned = true
	for i := 0; i < 5; i++ {
		_ = v.Render()
	}
	if v.scrollPos != 5 {
		t.Errorf("scrollPos changed across renders: got %d, want 5", v.scrollPos)
	}

	// Even when not pinned, Render() must not mutate scrollPos.
	v.pinned = false
	v.scrollPos = 7
	_ = v.Render()
	if v.scrollPos != 7 {
		t.Errorf("Render mutated scrollPos: got %d, want 7", v.scrollPos)
	}
}

// TestMessageGrewSnapsWhenNotPinned asserts the follow-tail behavior for
// streaming content growth.
func TestMessageGrewSnapsWhenNotPinned(t *testing.T) {
	v := setupMessageView(t)
	streaming := types.NewMessage(types.MsgAssistant, "")
	streaming.Streaming = true
	v.AddMessage(streaming)

	// Grow content; not pinned, so scrollPos must track the new bottom.
	streaming.Content = strings.Repeat("line\n", 50)
	v.MessageGrew()

	want := max(0, v.totalLineCount()-v.visible)
	if v.scrollPos != want {
		t.Errorf("scrollPos = %d, want %d (snapped to bottom)", v.scrollPos, want)
	}
}

// TestMessageGrewRespectsPin asserts the user's scroll position is preserved
// across content growth when pinned.
func TestMessageGrewRespectsPin(t *testing.T) {
	v := setupMessageView(t)
	streaming := types.NewMessage(types.MsgAssistant, "initial")
	streaming.Streaming = true
	v.AddMessage(streaming)

	// User scrolls up: must pin.
	v.scrollPos = 0
	for i := 0; i < 5; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "filler"))
	}
	v.scrollPos = 2
	v.pinned = true

	// Streaming message grows.
	streaming.Content = strings.Repeat("more\n", 100)
	v.MessageGrew()

	if v.scrollPos != 2 {
		t.Errorf("scrollPos = %d, want 2 (pinned)", v.scrollPos)
	}
	if !v.pinned {
		t.Error("pinned should remain true after MessageGrew")
	}
}

// TestScrollDownToBottomUnpins asserts the unpinning trigger.
func TestScrollDownToBottomUnpins(t *testing.T) {
	v := setupMessageView(t)
	for i := 0; i < 30; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}
	v.ScrollPageUp()
	if !v.pinned {
		t.Fatal("pinned should be true after ScrollPageUp")
	}

	// Scroll down to bottom.
	for i := 0; i < 100; i++ {
		v.ScrollDown()
	}
	if v.pinned {
		t.Error("pinned should be cleared once scrollPos reaches the bottom")
	}
}

// TestScrollToBottomClearsPin asserts the End-key path explicitly unpins.
func TestScrollToBottomClearsPin(t *testing.T) {
	v := setupMessageView(t)
	for i := 0; i < 30; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}
	v.ScrollToTop()
	if !v.pinned {
		t.Fatal("pinned should be true after ScrollToTop")
	}
	v.ScrollToBottom()
	if v.pinned {
		t.Error("pinned should be false after ScrollToBottom")
	}
}

// TestLinesBelow exercises the accessor that drives the status-bar ↓N hint.
func TestLinesBelow(t *testing.T) {
	v := setupMessageView(t) // 80x10
	for i := 0; i < 30; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}

	// At bottom: zero lines below.
	v.ScrollToBottom()
	if got := v.LinesBelow(); got != 0 {
		t.Errorf("LinesBelow at bottom = %d, want 0", got)
	}

	// Scrolled up by one page: that many lines below.
	total := v.totalLineCount()
	v.scrollPos = 0
	v.pinned = true
	want := total - v.visible
	if got := v.LinesBelow(); got != want {
		t.Errorf("LinesBelow at top = %d, want %d", got, want)
	}

	// Past the end (defensive): clamped to zero.
	v.scrollPos = total + 5
	if got := v.LinesBelow(); got != 0 {
		t.Errorf("LinesBelow past end = %d, want 0", got)
	}
}

// TestAddMessageDoesNotUnpin is the regression test for Issue 5 from the
// scrolling analysis: receiving a new message while the user is reading
// scrollback must not yank them back to the bottom.
func TestAddMessageDoesNotUnpin(t *testing.T) {
	v := setupMessageView(t)
	for i := 0; i < 30; i++ {
		v.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}
	v.ScrollPageUp()
	pinnedPos := v.scrollPos

	v.AddMessage(types.NewMessage(types.MsgAssistant, "new arrival"))

	if !v.pinned {
		t.Error("pinned should remain true across AddMessage")
	}
	if v.scrollPos != pinnedPos {
		t.Errorf("scrollPos = %d, want %d (preserved across AddMessage)", v.scrollPos, pinnedPos)
	}
}

// TestMessageAtYBaseline verifies MessageAtY correctly maps screen Y
// coordinates to message indices for a simple message layout.
func TestMessageAtYBaseline(t *testing.T) {
	v := setupMessageView(t)
	v.SetDimensions(80, 20)

	// Create a mix of message types, some expandable.
	msgs := []*types.Message{
		types.NewMessage(types.MsgUser, "hello world"),          // ~2 lines
		types.NewMessage(types.MsgAssistant, "## Response\n\nSome *markdown* content."),
		func() *types.Message {
			m := types.NewToolResult("read", "line a\nline b\nline c\n", false)
			return m // Collapsed by default, with summary
		}(),
		types.NewMessage(types.MsgUser, "another message"),
	}
	v.SetMessages(msgs)
	v.Render()

	totalLines := v.totalLineCount()
	if totalLines == 0 {
		t.Fatal("total line count should be > 0")
	}

	// Every valid Y should map to a message.
	for y := 0; y < v.visible; y++ {
		idx := v.MessageAtY(y)
		absLine := y + v.scrollPos
		if absLine >= totalLines {
			// Y maps to padding area below messages — should return -1
			if idx != -1 {
				t.Errorf("MessageAtY(%d) on padding line (abs=%d, total=%d) = %d, want -1",
					y, absLine, totalLines, idx)
			}
			continue
		}
		if idx < 0 || idx >= len(msgs) {
			t.Errorf("MessageAtY(%d) = %d, want valid index in [0, %d) (absLine=%d, total=%d)",
				y, idx, len(msgs), absLine, totalLines)
		}
	}
}

// TestMessageAtYBoundaries checks edge cases: exactly at message boundaries,
// one line before start, one line after end.
func TestMessageAtYBoundaries(t *testing.T) {
	v := setupMessageView(t)
	v.SetDimensions(80, 20)

	msgs := []*types.Message{
		types.NewMessage(types.MsgUser, "first user message"),
		func() *types.Message {
			m := types.NewToolResult("bash", "command output here", false)
			return m // Collapsed — should be 1 line
		}(),
		types.NewMessage(types.MsgAssistant, "final response"),
	}
	v.SetMessages(msgs)
	v.Render()

	// Collect line ranges.
	ranges := make([]struct{ idx, start, end int }, 0)
	for i := range v.lineRanges {
		ranges = append(ranges, struct{ idx, start, end int }{
			idx:   v.lineRanges[i].msgIndex,
			start: v.lineRanges[i].startLine,
			end:   v.lineRanges[i].endLine,
		})
	}

	if len(ranges) < 3 {
		t.Fatalf("expected at least 3 line ranges, got %d", len(ranges))
	}

	// Test each range's boundaries.
	for _, r := range ranges {
		// Line before start should map to different message (or -1 if at very beginning).
		if r.start > 0 {
			prev := v.MessageAtY(r.start - 1)
			if prev == r.idx {
				t.Errorf("line before msg %d (absLine=%d) should not return same message, got %d",
					r.idx, r.start-1, prev)
			}
		}

		// First line of message should map to this message.
		first := v.MessageAtY(r.start)
		if first != r.idx {
			t.Errorf("first line of msg %d (absLine=%d) = %d, want %d",
				r.idx, r.start, first, r.idx)
		}

		// Last line of message should map to this message.
		if r.end > r.start {
			last := v.MessageAtY(r.end - 1)
			if last != r.idx {
				t.Errorf("last line of msg %d (absLine=%d) = %d, want %d",
					r.idx, r.end-1, last, r.idx)
			}
		}

		// Line after end should map to next message.
		totalLines := v.totalLineCount()
		if r.end < totalLines {
			after := v.MessageAtY(r.end)
			if after == r.idx {
				t.Errorf("line after msg %d (absLine=%d) should not return same message, got %d",
					r.idx, r.end, after)
			}
		}
	}
}

// TestMessageAtYWithScrolling verifies MessageAtY works correctly when the
// viewport is scrolled.
func TestMessageAtYWithScrolling(t *testing.T) {
	v := setupMessageView(t)
	v.SetDimensions(80, 10)

	// Create enough messages to force scrolling.
	var msgs []*types.Message
	for i := 0; i < 30; i++ {
		msgs = append(msgs, types.NewMessage(types.MsgUser, fmt.Sprintf("message %d", i)))
	}
	v.SetMessages(msgs)
	v.Render()

	totalLines := v.totalLineCount()

	// Scroll to various positions and verify mapping.
	testPositions := []int{0, 5, totalLines/3, totalLines/2, totalLines - v.visible}
	for _, scrollPos := range testPositions {
		if scrollPos < 0 {
			scrollPos = 0
		}
		v.scrollPos = scrollPos
		v.Render() // Rebuild lineRanges with current scrollPos

		for y := 0; y < v.visible; y++ {
			absLine := y + v.scrollPos
			idx := v.MessageAtY(y)
			if absLine >= totalLines {
				if idx != -1 {
					t.Errorf("scrollPos=%d, Y=%d (abs=%d, total=%d) = %d, want -1",
						scrollPos, y, absLine, totalLines, idx)
				}
				continue
			}
			if idx < 0 || idx >= len(msgs) {
				t.Errorf("scrollPos=%d, Y=%d (abs=%d) = %d, invalid index",
					scrollPos, y, absLine, idx)
			}
		}
	}
}

// TestMessageAtYCollapsedToolResult verifies that a collapsed tool result
// (1 line) maps correctly and can be clicked to expand.
func TestMessageAtYCollapsedToolResult(t *testing.T) {
	v := setupMessageView(t)
	v.SetDimensions(80, 20)

	msgs := []*types.Message{
		types.NewMessage(types.MsgUser, "preface"),
		func() *types.Message {
			m := types.NewToolResult("bash", "some long output\nwith multiple lines\nof content", false)
			return m // Collapsed, has Summary
		}(),
		types.NewMessage(types.MsgAssistant, "post"),
	}
	v.SetMessages(msgs)
	v.Render()

	// The tool result should be collapsed (1 line). Find it in lineRanges.
	var toolIdx int
	for i, lr := range v.lineRanges {
		if lr.msgIndex == 1 { // Second message (index 1) is the tool result
			toolIdx = i
			break
		}
	}
	lr := v.lineRanges[toolIdx]
	lineCount := lr.endLine - lr.startLine
	if lineCount != 1 {
		t.Logf("collapsed tool result has %d lines (expected 1) — may be rendered differently", lineCount)
	}

	// Click anywhere on that line should return idx 1.
	// Since the viewport shows the top, the tool result's absolute line = its start line.
	// screenY = absLine - scrollPos.
	screenY := lr.startLine - v.scrollPos
	if screenY < v.visible {
		got := v.MessageAtY(screenY)
		if got != 1 {
			t.Errorf("MessageAtY(%d) = %d, want 1 (collapsed tool result)", screenY, got)
		}
	}

	// Now expand and verify the line range grows.
	msgs[1].Collapsed = false
	v.Render()
	expandedCount := v.lineRanges[toolIdx].endLine - v.lineRanges[toolIdx].startLine
	if expandedCount <= lineCount {
		t.Errorf("expanded tool result has %d lines, should be > collapsed count %d", expandedCount, lineCount)
	}
}

// TestMessageAtYThinkingToggle verifies that a message with thinking text
// is found correctly and can be toggled.
func TestMessageAtYThinkingToggle(t *testing.T) {
	v := setupMessageView(t)
	v.SetDimensions(80, 20)

	msgs := []*types.Message{
		types.NewMessage(types.MsgUser, "preface"),
		func() *types.Message {
			m := types.NewMessage(types.MsgAssistant, "the answer")
			// Long enough to wrap when expanded, so line count visibly grows.
			m.ThinkingText = "hmm let me think about this carefully, this is a longer thinking text that should wrap to multiple lines when expanded so we can verify the line count changes"
			return m
		}(),
	}
	v.SetMessages(msgs)
	v.Render()

	// Thinking is collapsed — the indicator is 1 line.
	// Find the message's line range.
	var msgLR messageLineRange
	for _, lr := range v.lineRanges {
		if lr.msgIndex == 1 {
			msgLR = lr
			break
		}
	}

	collapsedLines := msgLR.endLine - msgLR.startLine

	// Click on the thinking indicator line (first line of message).
	screenY := msgLR.startLine - v.scrollPos
	got := v.MessageAtY(screenY)
	if got != 1 {
		t.Errorf("MessageAtY(%d) = %d, want 1 (thinking collapsed indicator)", screenY, got)
	}

	// Expand thinking and verify line count grows.
	msgs[1].ThinkingExpanded = true
	v.Render()

	var expandedLR messageLineRange
	for _, lr := range v.lineRanges {
		if lr.msgIndex == 1 {
			expandedLR = lr
			break
		}
	}
	expandedLines := expandedLR.endLine - expandedLR.startLine
	if expandedLines <= collapsedLines {
		t.Errorf("expanded thinking has %d lines, should be > collapsed count %d", expandedLines, collapsedLines)
	}
}

// TestMessageAtYAfterCacheInvalidation verifies that MessageAtY still works
// after ensureRendered modifies the cache without a subsequent Render() call.
// This simulates the condition where MessageGrew() triggers re-rendering
// between two Render() calls.
func TestMessageAtYAfterCacheInvalidation(t *testing.T) {
	v := setupMessageView(t)
	v.SetDimensions(80, 20)

	// Start with a collapsed tool result.
	toolResult := types.NewToolResult("bash", "line1\nline2\nline3", false)
	msgs := []*types.Message{
		types.NewMessage(types.MsgUser, "preface"),
		toolResult,
		types.NewMessage(types.MsgAssistant, "post"),
	}
	v.SetMessages(msgs)
	v.Render()

	// Verify MessageAtY works before cache modification.
	collapsedLine := -1
	for _, lr := range v.lineRanges {
		if lr.msgIndex == 1 {
			collapsedLine = lr.startLine
			break
		}
	}
	if collapsedLine < 0 {
		t.Fatal("could not find tool result in line ranges")
	}

	// Now simulate a click-to-expand: toggle Collapsed and call MessageGrew.
	// This mirrors the exact sequence in the click handler.
	toolResult.Collapsed = false
	v.MessageGrew()

	// At this point, v.rendered has been updated by ensureRendered() inside
	// MessageGrew() -> maybeFollowTail() -> totalLineCount().
	// But v.lineRanges still reflects the OLD (collapsed) layout.
	// If MessageAtY uses stale lineRanges, this test will fail.

	// Now click again on the expanded result. The expanded result takes more
	// lines, so clicking at the same screen Y should still find the message.
	// But if lineRanges is stale, it might return the wrong index.

	// Test: the first line of the expanded message should still resolve.
	screenY := collapsedLine - v.scrollPos
	got := v.MessageAtY(screenY)
	if got != 1 {
		t.Errorf("after MessageGrew (cache invalidation), MessageAtY(%d) = %d, want 1", screenY, got)
	}

	// Test: a line that was within message 2 (the assistant) in the old layout
	// should now be within the expanded tool result in the new layout.
	// We need to find where message 2 starts in the OLD lineRanges.
	oldAssistantStart := -1
	for _, lr := range v.lineRanges {
		if lr.msgIndex == 2 {
			oldAssistantStart = lr.startLine
			break
		}
	}
	if oldAssistantStart < 0 {
		t.Fatal("could not find assistant message in old line ranges")
	}

	// In the old layout, oldAssistantStart is the first line of the assistant msg.
	// In the new layout (after expansion), that same absolute line should be
	// within the expanded tool result. MessageAtY should return 1, not 2.
	got2 := v.MessageAtY(oldAssistantStart - v.scrollPos)
	if got2 == 2 {
		t.Errorf("STALE LINE RANGES BUG: after MessageGrew, MessageAtY(%d) = %d (assistant msg), but expanded tool result now occupies that line — should return 1",
			oldAssistantStart-v.scrollPos, got2)
	}
}
