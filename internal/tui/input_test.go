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

func TestInputAutocompleteAutoActivateOnSlash(t *testing.T) {
	input := setupInput(t)

	// Register some commands
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{"gemma4", "qwen3.5"},
	})
	registry.Register(CommandDefinition{
		Name:        "debug",
		Description: "Toggle debug",
		HasArgs:     true,
		ArgValues:   []string{"on", "off"},
	})
	registry.Register(CommandDefinition{
		Name:        "clear",
		Description: "Clear conversation",
		HasArgs:     false,
	})
	input.SetCommandRegistry(registry)

	// Test 1: Typing '/' at position (0,0) should activate autocomplete
	input.SetValue("")
	input.cursorRow = 0
	input.cursorCol = 0
	input.HandleRune('/')
	if !input.AutocompleteIsActive() {
		t.Error("Autocomplete should be active after typing '/' at position (0,0)")
	}
	suggestions := input.GetAutocompleteState().GetSuggestions()
	if len(suggestions) == 0 {
		t.Error("Should have suggestions after typing '/' at position (0,0)")
	}

	// Test 2: Typing '/' after space should NOT activate autocomplete (only at position 0,0)
	// First, reset autocomplete state from Test 1
	input.DismissAutocomplete()
	input.SetValue("test ")
	input.cursorRow = 0
	input.cursorCol = len([]rune("test "))
	input.HandleRune('/')
	if input.AutocompleteIsActive() {
		t.Error("Autocomplete should NOT be active after typing '/' after space (only at position 0,0)")
	}

	// Test 3: Typing '/' in the middle of a word should NOT activate autocomplete
	input.SetValue("test")
	input.cursorRow = 0
	input.cursorCol = 2 // cursor in middle of "test"
	input.HandleRune('/')
	// Note: This will insert '/' making it "te/st", but autocomplete should not activate
	// because '/' was not at position (0,0)
}

func TestInputAutocompleteRemainsInactiveOnRegularChars(t *testing.T) {
	input := setupInput(t)

	// Register commands
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
	})
	input.SetCommandRegistry(registry)

	// Typing regular characters should not activate autocomplete
	input.SetValue("")
	input.HandleRune('h')
	if input.AutocompleteIsActive() {
		t.Error("Autocomplete should not be active after typing regular character")
	}
}

func TestInputAcceptAutocompleteBoundsCheck(t *testing.T) {
	input := setupInput(t)

	// Register commands
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{"gemma4", "qwen3.5"},
	})
	input.SetCommandRegistry(registry)

	// Setup: activate autocomplete with a longer text
	input.SetValue("/model gemma4")
	input.cursorRow = 0
	input.cursorCol = len("/model gemma4")

	// Manually activate autocomplete to simulate the state
	// (normally this happens via TryActivateAutocomplete)
	input.autocomplete.ActivateCommandCompletion("", 0, registry)

	// Simulate the bug scenario: text gets shorter but insertPos stays the same
	// This can happen if user deletes characters after autocomplete activates
	input.SetValue("x") // Short text, but autocomplete still has old insertPos

	// This should not panic even though insertPos (0) is now valid
	// but the scenario tests that we handle edge cases
	input.AcceptAutocomplete()

	// Verify no crash occurred and autocomplete is deactivated
	if input.AutocompleteIsActive() {
		t.Error("Autocomplete should be deactivated after AcceptAutocomplete")
	}
}

func TestInputAcceptAutocompleteInsertPosExceedsText(t *testing.T) {
	input := setupInput(t)

	// Register commands
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{"gemma4", "qwen3.5"},
	})
	input.SetCommandRegistry(registry)

	// Setup: activate autocomplete
	input.SetValue("/model")
	input.cursorRow = 0
	input.cursorCol = len("/model")

	// Activate command completion
	input.TryActivateAutocomplete()

	// Simulate the panic scenario: user deletes characters, making text shorter
	// but autocomplete state still has the old insertPos
	// Manually set a large insertPos to simulate the bug
	input.autocomplete.insertPos = 100 // Way beyond text length

	// This should not panic - the fix should clamp insertPos to len(text)
	input.AcceptAutocomplete()

	// Verify no crash and autocomplete is deactivated
	if input.AutocompleteIsActive() {
		t.Error("Autocomplete should be deactivated after AcceptAutocomplete")
	}

	// The text should have been modified (even if just appending at end)
	// since we accepted a suggestion
}

func TestInputHandleKeyEscDismissesAutocomplete(t *testing.T) {
	input := setupInput(t)

	// Register some commands
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{"gemma4", "qwen3.5"},
	})
	registry.Register(CommandDefinition{
		Name:        "debug",
		Description: "Toggle debug",
		HasArgs:     true,
		ArgValues:   []string{"on", "off"},
	})
	input.SetCommandRegistry(registry)

	// Activate autocomplete by typing '/' at position (0,0)
	input.SetValue("")
	input.cursorRow = 0
	input.cursorCol = 0
	input.HandleRune('/')

	if !input.AutocompleteIsActive() {
		t.Fatal("Autocomplete should be active after typing '/'")
	}

	// Press ESC to dismiss autocomplete
	handled := input.HandleKey(tea.KeyEsc)

	if !handled {
		t.Error("HandleKey should return true for ESC when autocomplete is active")
	}

	if input.AutocompleteIsActive() {
		t.Error("Autocomplete should be deactivated after pressing ESC")
	}
}

func TestInputHandleKeyEscWhenNotFocused(t *testing.T) {
	input := setupInput(t)
	input.focused = false

	// Autocomplete should not be active
	input.SetValue("/model")

	handled := input.HandleKey(tea.KeyEsc)

	if handled {
		t.Error("HandleKey should return false when input is not focused")
	}
}

func TestInputHandleKeyEscExitsHistoryMode(t *testing.T) {
	input := setupInput(t)

	// Setup: put input in history mode
	input.historyMode = true
	input.historyIdx = -1
	input.originalValue = ""

	handled := input.HandleKey(tea.KeyEsc)

	if !handled {
		t.Error("HandleKey should return true for ESC in history mode")
	}

	if input.historyMode {
		t.Error("historyMode should be false after pressing ESC")
	}
}
