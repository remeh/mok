package tui

import (
	"testing"

	"github.com/charmbracelet/bubbletea"
)

func setupInput(t *testing.T) *InputArea {
	t.Helper()
	return NewInputArea(DefaultTheme(), ">")
}

func TestInputValue(t *testing.T) {
	input := setupInput(t)

	input.SetValue("hello")
	if input.Value() != "hello" {
		t.Errorf("Value() = %q, want 'hello'", input.Value())
	}
}

func TestInputHandleRune(t *testing.T) {
	input := setupInput(t)

	input.HandleRune('h')
	input.HandleRune('i')
	if input.Value() != "hi" {
		t.Errorf("Value() = %q, want 'hi'", input.Value())
	}
}

func TestInputHandleRuneInsertion(t *testing.T) {
	input := setupInput(t)

	input.SetValue("hello")
	// Simulate cursor at position 2 (after "he")
	input.cursorPos = 2

	input.HandleRune('X')
	if input.Value() != "heXllo" {
		t.Errorf("Value() = %q, want 'heXllo'", input.Value())
	}
	if input.cursorPos != 3 {
		t.Errorf("cursorPos = %d, want 3", input.cursorPos)
	}
}

func TestInputHandleKeyBackspace(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 3 // after "hel"

	handled := input.HandleKey(tea.KeyBackspace)
	if !handled {
		t.Fatal("HandleKey should return true for backspace")
	}
	if input.Value() != "helo" {
		t.Errorf("Value() = %q, want 'helo'", input.Value())
	}
	if input.cursorPos != 2 {
		t.Errorf("cursorPos = %d, want 2", input.cursorPos)
	}
}

func TestInputHandleKeyBackspaceAtStart(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 0

	handled := input.HandleKey(tea.KeyBackspace)
	if !handled {
		t.Fatal("HandleKey should return true for backspace")
	}
	if input.Value() != "hello" {
		t.Errorf("Value() = %q, should be unchanged", input.Value())
	}
	if input.cursorPos != 0 {
		t.Errorf("cursorPos = %d, want 0", input.cursorPos)
	}
}

func TestInputHandleKeyDelete(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 2 // after "he"

	handled := input.HandleKey(tea.KeyDelete)
	if !handled {
		t.Fatal("HandleKey should return true for delete")
	}
	if input.Value() != "helo" {
		t.Errorf("Value() = %q, want 'helo'", input.Value())
	}
	if input.cursorPos != 2 {
		t.Errorf("cursorPos = %d, want 2", input.cursorPos)
	}
}

func TestInputHandleKeyDeleteAtEnd(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 5

	handled := input.HandleKey(tea.KeyDelete)
	if !handled {
		t.Fatal("HandleKey should return true for delete")
	}
	if input.Value() != "hello" {
		t.Errorf("Value() = %q, should be unchanged", input.Value())
	}
}

func TestInputHandleKeyLeft(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 3

	handled := input.HandleKey(tea.KeyLeft)
	if !handled {
		t.Fatal("HandleKey should return true for left")
	}
	if input.cursorPos != 2 {
		t.Errorf("cursorPos = %d, want 2", input.cursorPos)
	}
}

func TestInputHandleKeyLeftAtStart(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 0

	handled := input.HandleKey(tea.KeyLeft)
	if !handled {
		t.Fatal("HandleKey should return true for left")
	}
	if input.cursorPos != 0 {
		t.Errorf("cursorPos = %d, should stay 0", input.cursorPos)
	}
}

func TestInputHandleKeyRight(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 2

	handled := input.HandleKey(tea.KeyRight)
	if !handled {
		t.Fatal("HandleKey should return true for right")
	}
	if input.cursorPos != 3 {
		t.Errorf("cursorPos = %d, want 3", input.cursorPos)
	}
}

func TestInputHandleKeyRightAtEnd(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 5

	handled := input.HandleKey(tea.KeyRight)
	if !handled {
		t.Fatal("HandleKey should return true for right")
	}
	if input.cursorPos != 5 {
		t.Errorf("cursorPos = %d, should stay 5", input.cursorPos)
	}
}

func TestInputHandleKeyCtrlA(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 3

	handled := input.HandleKey(tea.KeyCtrlA)
	if !handled {
		t.Fatal("HandleKey should return true for Ctrl+A")
	}
	if input.cursorPos != 0 {
		t.Errorf("cursorPos = %d, want 0 (home)", input.cursorPos)
	}
}

func TestInputHandleKeyCtrlE(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorPos = 1

	handled := input.HandleKey(tea.KeyCtrlE)
	if !handled {
		t.Fatal("HandleKey should return true for Ctrl+E")
	}
	if input.cursorPos != 5 {
		t.Errorf("cursorPos = %d, want 5 (end)", input.cursorPos)
	}
}

func TestInputHandleKeyCtrlW(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello world")
	input.cursorPos = 11 // at end

	handled := input.HandleKey(tea.KeyCtrlW)
	if !handled {
		t.Fatal("HandleKey should return true for Ctrl+W")
	}
	if input.Value() != "hello " {
		t.Errorf("Value() = %q, want 'hello '", input.Value())
	}
	if input.cursorPos != 6 {
		t.Errorf("cursorPos = %d, want 6", input.cursorPos)
	}
}

func TestInputHandleKeyCtrlU(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello world")
	input.cursorPos = 5

	handled := input.HandleKey(tea.KeyCtrlU)
	if !handled {
		t.Fatal("HandleKey should return true for Ctrl+U")
	}
	if input.Value() != "" {
		t.Errorf("Value() = %q, want empty", input.Value())
	}
	if input.cursorPos != 0 {
		t.Errorf("cursorPos = %d, want 0", input.cursorPos)
	}
}

func TestInputHandleKeyEnter(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")

	handled := input.HandleKey(tea.KeyEnter)
	if !handled {
		t.Fatal("HandleKey should return true for Enter")
	}
}

func TestInputHistory(t *testing.T) {
	input := setupInput(t)

	// Add items to history
	input.SetValue("first")
	input.PushHistory()
	input.SetValue("second")
	input.PushHistory()
	input.SetValue("third")
	input.PushHistory()

	// Navigate up
	input.HandleKey(tea.KeyUp)
	if input.Value() != "third" {
		t.Errorf("Value() = %q, want 'third'", input.Value())
	}

	input.HandleKey(tea.KeyUp)
	if input.Value() != "second" {
		t.Errorf("Value() = %q, want 'second'", input.Value())
	}

	// Navigate down
	input.HandleKey(tea.KeyDown)
	if input.Value() != "third" {
		t.Errorf("Value() = %q, want 'third'", input.Value())
	}

	input.HandleKey(tea.KeyDown)
	if input.Value() != "" {
		t.Errorf("Value() = %q, want empty (cleared after last down)", input.Value())
	}
}

func TestInputHistoryEmpty(t *testing.T) {
	input := setupInput(t)

	handled := input.HandleKey(tea.KeyUp)
	if !handled {
		t.Fatal("HandleKey should return true even with empty history")
	}
	if input.Value() != "" {
		t.Errorf("Value() = %q, want empty", input.Value())
	}
}

func TestInputPushHistoryEmpty(t *testing.T) {
	input := setupInput(t)
	input.SetValue("")
	input.PushHistory()

	if len(input.history) != 0 {
		t.Errorf("history length = %d, want 0 (empty values should not be added)", len(input.history))
	}
}

func TestInputNotFocused(t *testing.T) {
	input := setupInput(t)
	input.focused = false

	// Should not handle any keys when not focused
	input.HandleRune('h')
	if input.Value() != "" {
		t.Errorf("Value() = %q, want empty (not focused)", input.Value())
	}

	handled := input.HandleKey(tea.KeyBackspace)
	if handled {
		t.Error("HandleKey should return false when not focused")
	}
}

func TestInputSetWidth(t *testing.T) {
	input := setupInput(t)
	input.SetWidth(100)

	if input.width != 100 {
		t.Errorf("width = %d, want 100", input.width)
	}
}

func TestInputRender(t *testing.T) {
	input := setupInput(t)
	input.SetWidth(80)
	input.SetValue("hello")

	rendered := input.Render()
	if rendered == "" {
		t.Error("Render should not return empty string")
	}
}