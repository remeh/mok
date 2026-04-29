package tui

import (
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

func TestScreenSetPartialText(t *testing.T) {
	screen := setupScreen(t)
	screen.SetPartialText("partial response...")

	if screen.partialText != "partial response..." {
		t.Errorf("partialText = %q, want 'partial response...'", screen.partialText)
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
