package tui

import (
	"testing"
)

func TestCommandRegistryComplete(t *testing.T) {
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{"gemma4-e4b", "qwen3.5-9b-thinking"},
	})
	registry.Register(CommandDefinition{
		Name:        "debug",
		Description: "Toggle debug mode",
		HasArgs:     true,
		ArgValues:   []string{"on", "off"},
	})
	registry.Register(CommandDefinition{
		Name:        "clear",
		Description: "Clear conversation",
		HasArgs:     false,
	})

	// Test all commands
	commands := registry.Complete("")
	if len(commands) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(commands))
	}

	// Test prefix matching
	commands = registry.Complete("mo")
	if len(commands) != 1 || commands[0] != "model" {
		t.Errorf("Expected ['model'], got %v", commands)
	}

	commands = registry.Complete("cl")
	if len(commands) != 1 || commands[0] != "clear" {
		t.Errorf("Expected ['clear'], got %v", commands)
	}
}

func TestCommandRegistryCompleteValues(t *testing.T) {
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{"gemma4-e4b", "qwen3.5-9b-thinking"},
	})
	registry.Register(CommandDefinition{
		Name:        "debug",
		Description: "Toggle debug mode",
		HasArgs:     true,
		ArgValues:   []string{"on", "off"},
	})

	// Test all values
	values := registry.CompleteValues("model", "")
	if len(values) != 2 {
		t.Errorf("Expected 2 model values, got %d", len(values))
	}

	// Test prefix matching
	values = registry.CompleteValues("model", "q")
	if len(values) != 1 || values[0] != "qwen3.5-9b-thinking" {
		t.Errorf("Expected ['qwen3.5-9b-thinking'], got %v", values)
	}

	values = registry.CompleteValues("debug", "o")
	if len(values) != 2 {
		t.Errorf("Expected 2 values starting with 'o', got %d: %v", len(values), values)
	}
}

func TestGetPrefixAtCursor(t *testing.T) {
	// Test command completion
	prefix, cmdName, insertPos, compType := GetPrefixAtCursor("/mo", 3)
	if compType != CompletionCommand {
		t.Errorf("Expected CompletionCommand, got %d", compType)
	}
	if prefix != "mo" {
		t.Errorf("Expected prefix 'mo', got '%s'", prefix)
	}
	if insertPos != 1 {
		t.Errorf("Expected insertPos 1, got %d", insertPos)
	}

	// Test value completion
	prefix, cmdName, insertPos, compType = GetPrefixAtCursor("/model gem", 10)
	if compType != CompletionValue {
		t.Errorf("Expected CompletionValue, got %d", compType)
	}
	if prefix != "gem" {
		t.Errorf("Expected prefix 'gem', got '%s'", prefix)
	}
	if cmdName != "model" {
		t.Errorf("Expected cmdName 'model', got '%s'", cmdName)
	}
}

func TestAutocompleteState(t *testing.T) {
	registry := NewCommandRegistry()
	registry.Register(CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{"gemma4-e4b", "qwen3.5-9b-thinking"},
	})
	registry.Register(CommandDefinition{
		Name:        "debug",
		Description: "Toggle debug mode",
		HasArgs:     true,
		ArgValues:   []string{"on", "off"},
	})

	state := NewAutocompleteState()

	// Test command completion activation
	state.ActivateCommandCompletion("mo", 3, registry)
	if !state.IsActive() {
		t.Error("Expected autocomplete to be active")
	}
	if len(state.suggestions) != 1 || state.suggestions[0] != "model" {
		t.Errorf("Expected ['model'], got %v", state.suggestions)
	}

	// Test selection navigation
	state.SelectNext()
	if state.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex 0 (only one item), got %d", state.selectedIndex)
	}

	// Test deactivation
	state.Deactivate()
	if state.IsActive() {
		t.Error("Expected autocomplete to be deactivated")
	}
}
