package tui

import (
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

// TestStreamingLineCountStableAcrossBlinkFrames is the Phase 1 invariant:
// during streaming, the cursor blink must not change the total line count.
// Before Phase 1 the streaming cursor lived on its own row that flickered
// on/off, and messageLineCount() didn't model it, so scroll math drifted.
func TestStreamingLineCountStableAcrossBlinkFrames(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewMessage(types.MsgAssistant, "streaming content here")
	msg.Streaming = true
	v.AddMessage(msg)

	counts := map[int]bool{}
	for frame := 0; frame < 16; frame++ {
		v.cursorFrame = frame
		counts[v.totalLineCount()] = true
	}
	if len(counts) != 1 {
		t.Errorf("totalLineCount varies across cursor frames: %v", counts)
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
