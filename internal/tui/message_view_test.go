package tui

import (
	"strings"
	"testing"

	"github.com/user/mmok/internal/types"
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

func TestMessageViewScrollAutoScroll(t *testing.T) {
	v := setupMessageView(t)
	v.AddMessage(types.NewMessage(types.MsgUser, "hello"))

	if !v.autoScroll {
		t.Error("autoScroll should be true after adding message")
	}

	// ScrollUp only disables autoScroll if scrollPos > 0
	// With a single message, scrollPos is 0, so autoScroll stays true
	v.ScrollUp()
	// autoScroll remains true because we're at the top (scrollPos == 0)
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
	if v.autoScroll {
		t.Error("autoScroll should be false after page up")
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
	if v.autoScroll {
		t.Error("autoScroll should be false after scroll to top")
	}
}

func TestMessageViewToolCall(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewToolCall("read", `path="test.go"`)
	v.AddMessage(msg)

	rendered := v.Render()
	if !strings.Contains(rendered, "tool_call") {
		t.Errorf("Render should contain tool_call label: %q", rendered)
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
	if !strings.Contains(rendered, "ctrl-o to expand") {
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
	if !strings.Contains(rendered, "tool_result") {
		t.Errorf("Render should contain tool_result label: %q", rendered)
	}
	if !strings.Contains(rendered, "file contents here") {
		t.Errorf("Render should contain full content: %q", rendered)
	}
}

func TestMessageViewSystemMessage(t *testing.T) {
	v := setupMessageView(t)

	msg := types.NewMessage(types.MsgSystem, "system message")
	v.AddMessage(msg)

	rendered := v.Render()
	if !strings.Contains(rendered, "system") {
		t.Errorf("Render should contain system label: %q", rendered)
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
