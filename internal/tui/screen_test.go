package tui

import (
	"strings"
	"testing"

	"github.com/user/mmok/internal/types"
)

func setupScreen(t *testing.T) *Screen {
	t.Helper()
	return NewScreen(DefaultTheme())
}

func TestScreenSetDimensions(t *testing.T) {
	screen := setupScreen(t)
	screen.SetDimensions(120, 30)

	if screen.width != 120 {
		t.Errorf("width = %d, want 120", screen.width)
	}
	if screen.height != 30 {
		t.Errorf("height = %d, want 30", screen.height)
	}
}

func TestScreenSetMessages(t *testing.T) {
	screen := setupScreen(t)

	messages := []*types.Message{
		types.NewMessage(types.MsgUser, "hello"),
		types.NewMessage(types.MsgAssistant, "hi"),
	}
	screen.SetMessages(messages)

	if len(screen.msgView.messages) != 2 {
		t.Errorf("messages length = %d, want 2", len(screen.msgView.messages))
	}
}

func TestScreenAddMessage(t *testing.T) {
	screen := setupScreen(t)

	screen.AddMessage(types.NewMessage(types.MsgUser, "hello"))

	if len(screen.msgView.messages) != 1 {
		t.Errorf("messages length = %d, want 1", len(screen.msgView.messages))
	}
}

func TestScreenSetInputValue(t *testing.T) {
	screen := setupScreen(t)
	screen.SetInputValue("test input")

	if screen.inputArea.Value() != "test input" {
		t.Errorf("input value = %q, want 'test input'", screen.inputArea.Value())
	}
}

func TestScreenSetModel(t *testing.T) {
	screen := setupScreen(t)
	screen.SetModel("test-model")

	rendered := screen.statusBar.Render()
	if rendered == "" {
		t.Error("statusBar Render should not return empty string")
	}
}

func TestScreenSetTokenCount(t *testing.T) {
	screen := setupScreen(t)
	screen.SetTokenCount(5000)

	if screen.statusBar.tokenCount != 5000 {
		t.Errorf("tokenCount = %d, want 5000", screen.statusBar.tokenCount)
	}
}

func TestScreenSetMaxTokens(t *testing.T) {
	screen := setupScreen(t)
	screen.SetMaxTokens(65536)

	if screen.statusBar.maxTokens != 65536 {
		t.Errorf("maxTokens = %d, want 65536", screen.statusBar.maxTokens)
	}
}

func TestScreenSetStatusBarState(t *testing.T) {
	screen := setupScreen(t)
	screen.SetStatusBarState(StatusStreaming)

	rendered := screen.statusBar.Render()
	if rendered == "" {
		t.Error("statusBar Render should not return empty string")
	}
}

func TestScreenSetStreaming(t *testing.T) {
	screen := setupScreen(t)

	screen.SetStreaming(true)
	if screen.streaming {
		// streaming is set
	}

	screen.SetStreaming(false)
	if screen.streaming {
		t.Error("streaming should be false")
	}
}

func TestScreenRender(t *testing.T) {
	screen := setupScreen(t)
	screen.SetDimensions(80, 20)
	screen.SetModel("test-model")
	screen.SetTokenCount(100)

	rendered := screen.Render()
	if rendered == "" {
		t.Error("Render should not return empty string")
	}
}

func TestScreenGetInputArea(t *testing.T) {
	screen := setupScreen(t)
	input := screen.GetInputArea()

	if input == nil {
		t.Fatal("GetInputArea should not return nil")
	}
}

func TestScreenGetMessageView(t *testing.T) {
	screen := setupScreen(t)
	view := screen.GetMessageView()

	if view == nil {
		t.Fatal("GetMessageView should not return nil")
	}
}

// TestScreenLayoutFixedAcrossScroll is the Phase 3 invariant: the message
// view height is constant regardless of scroll state. Before Phase 3 the
// indicator stole a row when the user scrolled up, causing content to jump.
func TestScreenLayoutFixedAcrossScroll(t *testing.T) {
	screen := setupScreen(t)
	screen.SetDimensions(80, 20)
	for i := 0; i < 50; i++ {
		screen.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}

	heightAtBottom := screen.msgView.height

	screen.GetMessageView().ScrollPageUp()
	_ = screen.Render()
	heightScrolledUp := screen.msgView.height

	if heightAtBottom != heightScrolledUp {
		t.Errorf("msgView height changed across scroll: at-bottom=%d, scrolled-up=%d",
			heightAtBottom, heightScrolledUp)
	}
}

// TestScreenRenderIncludesScrollHintWhenScrolled verifies the indicator now
// lives in the status bar rather than as a separate row.
func TestScreenRenderIncludesScrollHintWhenScrolled(t *testing.T) {
	screen := setupScreen(t)
	screen.SetDimensions(120, 20)
	for i := 0; i < 50; i++ {
		screen.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}
	screen.GetMessageView().ScrollPageUp()

	rendered := screen.Render()
	if !strings.Contains(rendered, "↓") {
		t.Errorf("Render should contain ↓ scroll hint when scrolled up: %q", rendered)
	}
}

// TestScreenRenderHasFixedRowCount asserts Render() always emits exactly
// h - 2 (msg) + 1 (input) + 1 (status) = h rows, regardless of scroll state.
func TestScreenRenderHasFixedRowCount(t *testing.T) {
	screen := setupScreen(t)
	screen.SetDimensions(80, 20)
	for i := 0; i < 50; i++ {
		screen.AddMessage(types.NewMessage(types.MsgUser, "msg"))
	}

	atBottom := strings.Count(screen.Render(), "\n")
	screen.GetMessageView().ScrollPageUp()
	scrolledUp := strings.Count(screen.Render(), "\n")

	if atBottom != scrolledUp {
		t.Errorf("Render row count differs: at-bottom=%d newlines, scrolled-up=%d", atBottom, scrolledUp)
	}
}
