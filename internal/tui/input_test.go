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

func TestInputValueMultiLine(t *testing.T) {
	input := setupInput(t)

	input.SetValue("hello\nworld\nfoo")
	lines := input.GetLines()
	if len(lines) != 3 {
		t.Errorf("GetLines() length = %d, want 3", len(lines))
	}
	if lines[0] != "hello" {
		t.Errorf("lines[0] = %q, want 'hello'", lines[0])
	}
	if lines[1] != "world" {
		t.Errorf("lines[1] = %q, want 'world'", lines[1])
	}
	if lines[2] != "foo" {
		t.Errorf("lines[2] = %q, want 'foo'", lines[2])
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
	input.cursorRow = 0
	input.cursorCol = 2

	input.HandleRune('X')
	if input.Value() != "heXllo" {
		t.Errorf("Value() = %q, want 'heXllo'", input.Value())
	}
	if input.cursorCol != 3 {
		t.Errorf("cursorCol = %d, want 3", input.cursorCol)
	}
}

func TestInputHandleRuneNewline(t *testing.T) {
	input := setupInput(t)

	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 5 // at end

	input.HandleRune('\n')

	lines := input.GetLines()
	if len(lines) != 2 {
		t.Errorf("GetLines() length = %d, want 2", len(lines))
	}
	if lines[0] != "hello" {
		t.Errorf("lines[0] = %q, want 'hello'", lines[0])
	}
	if lines[1] != "" {
		t.Errorf("lines[1] = %q, want empty", lines[1])
	}
	if input.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", input.cursorRow)
	}
	if input.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0", input.cursorCol)
	}
}

func TestInputHandleKeyBackspace(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 3 // after "hel"

	handled := input.HandleKey(tea.KeyBackspace)
	if !handled {
		t.Fatal("HandleKey should return true for backspace")
	}
	if input.Value() != "helo" {
		t.Errorf("Value() = %q, want 'helo'", input.Value())
	}
	if input.cursorCol != 2 {
		t.Errorf("cursorCol = %d, want 2", input.cursorCol)
	}
}

func TestInputHandleKeyBackspaceAtStart(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 0

	handled := input.HandleKey(tea.KeyBackspace)
	if !handled {
		t.Fatal("HandleKey should return true for backspace")
	}
	if input.Value() != "hello" {
		t.Errorf("Value() = %q, should be unchanged", input.Value())
	}
	if input.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0", input.cursorCol)
	}
}

func TestInputHandleKeyBackspaceJoinLines(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello\nworld")
	input.cursorRow = 1
	input.cursorCol = 0 // at start of second line

	handled := input.HandleKey(tea.KeyBackspace)
	if !handled {
		t.Fatal("HandleKey should return true for backspace")
	}
	if input.Value() != "helloworld" {
		t.Errorf("Value() = %q, want 'helloworld'", input.Value())
	}
	if input.cursorRow != 0 {
		t.Errorf("cursorRow = %d, want 0", input.cursorRow)
	}
	if input.cursorCol != 5 {
		t.Errorf("cursorCol = %d, want 5", input.cursorCol)
	}
}

func TestInputHandleKeyDelete(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 2 // after "he"

	handled := input.HandleKey(tea.KeyDelete)
	if !handled {
		t.Fatal("HandleKey should return true for delete")
	}
	if input.Value() != "helo" {
		t.Errorf("Value() = %q, want 'helo'", input.Value())
	}
	if input.cursorCol != 2 {
		t.Errorf("cursorCol = %d, want 2", input.cursorCol)
	}
}

func TestInputHandleKeyDeleteAtEnd(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 5

	handled := input.HandleKey(tea.KeyDelete)
	if !handled {
		t.Fatal("HandleKey should return true for delete")
	}
	if input.Value() != "hello" {
		t.Errorf("Value() = %q, should be unchanged", input.Value())
	}
}

func TestInputHandleKeyDeleteJoinLines(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello\nworld")
	input.cursorRow = 0
	input.cursorCol = 5 // at end of first line

	handled := input.HandleKey(tea.KeyDelete)
	if !handled {
		t.Fatal("HandleKey should return true for delete")
	}
	if input.Value() != "helloworld" {
		t.Errorf("Value() = %q, want 'helloworld'", input.Value())
	}
	if input.cursorRow != 0 {
		t.Errorf("cursorRow = %d, want 0", input.cursorRow)
	}
	if input.cursorCol != 5 {
		t.Errorf("cursorCol = %d, want 5", input.cursorCol)
	}
}

func TestInputHandleKeyLeft(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 3

	handled := input.HandleKey(tea.KeyLeft)
	if !handled {
		t.Fatal("HandleKey should return true for left")
	}
	if input.cursorCol != 2 {
		t.Errorf("cursorCol = %d, want 2", input.cursorCol)
	}
}

func TestInputHandleKeyLeftAtStart(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 0

	handled := input.HandleKey(tea.KeyLeft)
	if !handled {
		t.Fatal("HandleKey should return true for left")
	}
	if input.cursorCol != 0 {
		t.Errorf("cursorCol = %d, should stay 0", input.cursorCol)
	}
}

func TestInputHandleKeyLeftToPrevLine(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello\nworld")
	input.cursorRow = 1
	input.cursorCol = 0

	handled := input.HandleKey(tea.KeyLeft)
	if !handled {
		t.Fatal("HandleKey should return true for left")
	}
	if input.cursorRow != 0 {
		t.Errorf("cursorRow = %d, want 0", input.cursorRow)
	}
	if input.cursorCol != 5 {
		t.Errorf("cursorCol = %d, want 5 (end of prev line)", input.cursorCol)
	}
}

func TestInputHandleKeyRight(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 2

	handled := input.HandleKey(tea.KeyRight)
	if !handled {
		t.Fatal("HandleKey should return true for right")
	}
	if input.cursorCol != 3 {
		t.Errorf("cursorCol = %d, want 3", input.cursorCol)
	}
}

func TestInputHandleKeyRightAtEnd(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 5

	handled := input.HandleKey(tea.KeyRight)
	if !handled {
		t.Fatal("HandleKey should return true for right")
	}
	if input.cursorCol != 5 {
		t.Errorf("cursorCol = %d, should stay 5", input.cursorCol)
	}
}

func TestInputHandleKeyRightToNextLine(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello\nworld")
	input.cursorRow = 0
	input.cursorCol = 5 // at end of first line

	handled := input.HandleKey(tea.KeyRight)
	if !handled {
		t.Fatal("HandleKey should return true for right")
	}
	if input.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", input.cursorRow)
	}
	if input.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0 (start of next line)", input.cursorCol)
	}
}

func TestInputHandleKeyUp(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello\nworld\nfoo")
	input.cursorRow = 2
	input.cursorCol = 3

	handled := input.HandleKey(tea.KeyUp)
	if !handled {
		t.Fatal("HandleKey should return true for up")
	}
	if input.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", input.cursorRow)
	}
	if input.cursorCol != 3 {
		t.Errorf("cursorCol = %d, want 3 (clamped to line length)", input.cursorCol)
	}
}

func TestInputHandleKeyUpClamped(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello\nw") // second line is shorter
	input.cursorRow = 1
	input.cursorCol = 10 // beyond second line length

	handled := input.HandleKey(tea.KeyUp)
	if !handled {
		t.Fatal("HandleKey should return true for up")
	}
	if input.cursorRow != 0 {
		t.Errorf("cursorRow = %d, want 0", input.cursorRow)
	}
	if input.cursorCol != 5 {
		t.Errorf("cursorCol = %d, want 5 (clamped to line length)", input.cursorCol)
	}
}

func TestInputHandleKeyUpAtStart(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 3

	handled := input.HandleKey(tea.KeyUp)
	if !handled {
		t.Fatal("HandleKey should return true for up")
	}
	if input.cursorRow != 0 {
		t.Errorf("cursorRow = %d, should stay 0", input.cursorRow)
	}
}

func TestInputHandleKeyDown(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello\nworld\nfoo")
	input.cursorRow = 0
	input.cursorCol = 3

	handled := input.HandleKey(tea.KeyDown)
	if !handled {
		t.Fatal("HandleKey should return true for down")
	}
	if input.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", input.cursorRow)
	}
	if input.cursorCol != 3 {
		t.Errorf("cursorCol = %d, want 3", input.cursorCol)
	}
}

func TestInputHandleKeyDownClamped(t *testing.T) {
	input := setupInput(t)
	input.SetValue("w\nhello") // first line is shorter
	input.cursorRow = 0
	input.cursorCol = 10 // beyond first line length

	handled := input.HandleKey(tea.KeyDown)
	if !handled {
		t.Fatal("HandleKey should return true for down")
	}
	if input.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", input.cursorRow)
	}
	if input.cursorCol != 5 {
		t.Errorf("cursorCol = %d, want 5 (clamped to line length)", input.cursorCol)
	}
}

func TestInputHandleKeyDownAtEnd(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 3

	handled := input.HandleKey(tea.KeyDown)
	if !handled {
		t.Fatal("HandleKey should return true for down")
	}
	if input.cursorRow != 0 {
		t.Errorf("cursorRow = %d, should stay 0", input.cursorRow)
	}
}

func TestInputHandleKeyCtrlA(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 3

	handled := input.HandleKey(tea.KeyCtrlA)
	if !handled {
		t.Fatal("HandleKey should return true for Ctrl+A")
	}
	if input.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0 (home)", input.cursorCol)
	}
}

func TestInputHandleKeyCtrlE(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 1

	handled := input.HandleKey(tea.KeyCtrlE)
	if !handled {
		t.Fatal("HandleKey should return true for Ctrl+E")
	}
	if input.cursorCol != 5 {
		t.Errorf("cursorCol = %d, want 5 (end)", input.cursorCol)
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

func TestInputHandleKeyEnterInsertsNewline(t *testing.T) {
	input := setupInput(t)
	input.SetValue("hello")
	input.cursorRow = 0
	input.cursorCol = 3 // after "hel"

	handled := input.HandleKey(tea.KeyEnter)
	if !handled {
		t.Fatal("HandleKey should return true for Enter")
	}

	lines := input.GetLines()
	if len(lines) != 2 {
		t.Errorf("GetLines() length = %d, want 2", len(lines))
	}
	if lines[0] != "hel" {
		t.Errorf("lines[0] = %q, want 'hel'", lines[0])
	}
	if lines[1] != "lo" {
		t.Errorf("lines[1] = %q, want 'lo'", lines[1])
	}
	if input.cursorRow != 1 {
		t.Errorf("cursorRow = %d, want 1", input.cursorRow)
	}
	if input.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0", input.cursorCol)
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

	// History navigation is not implemented in Phase 1
	// Up/Down arrows now navigate lines, not history
	// This test verifies history is stored correctly
	if len(input.history) != 3 {
		t.Errorf("history length = %d, want 3", len(input.history))
	}
}

func TestInputHistoryMultiLine(t *testing.T) {
	input := setupInput(t)

	// Add multi-line items to history
	input.SetValue("first\nline")
	input.PushHistory()
	input.SetValue("second\nline")
	input.PushHistory()

	// Navigate up
	input.HandleKey(tea.KeyUp)
	if input.Value() != "second\nline" {
		t.Errorf("Value() = %q, want 'second\\nline'", input.Value())
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

func TestInputPushHistoryMultiLineEmpty(t *testing.T) {
	input := setupInput(t)
	input.SetValue("\n")
	input.PushHistory()

	// Note: "\n" is not empty, it's a valid multi-line value with two empty lines
	// So it should be added to history
	if len(input.history) != 1 {
		t.Errorf("history length = %d, want 1", len(input.history))
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

func TestInputSetLines(t *testing.T) {
	input := setupInput(t)

	input.SetLines([]string{"hello", "world", "foo"})
	lines := input.GetLines()
	if len(lines) != 3 {
		t.Errorf("GetLines() length = %d, want 3", len(lines))
	}
	if lines[0] != "hello" {
		t.Errorf("lines[0] = %q, want 'hello'", lines[0])
	}
	if input.cursorRow != 0 {
		t.Errorf("cursorRow = %d, want 0", input.cursorRow)
	}
	if input.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0", input.cursorCol)
	}
}

func TestInputSetLinesEmpty(t *testing.T) {
	input := setupInput(t)

	input.SetLines([]string{})
	lines := input.GetLines()
	if len(lines) != 1 {
		t.Errorf("GetLines() length = %d, want 1 (should have single empty line)", len(lines))
	}
}
