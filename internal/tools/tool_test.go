package tools

import (
	"encoding/json"
	"testing"
)

// mockTool is a test tool implementation.
type mockTool struct {
	name        string
	description string
	parameters  map[string]any
	result      string
	shouldErr   bool
}

func (m *mockTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        m.name,
		Description: m.description,
		Parameters:  m.parameters,
	}
}

func (m *mockTool) Execute(args json.RawMessage) (string, error) {
	if m.shouldErr {
		return "", json.Unmarshal(args, &struct{}{}) // dummy error path
	}
	return m.result, nil
}

func TestRegistry_AddAndGet(t *testing.T) {
	reg := NewRegistry()
	tool := &mockTool{name: "test_tool", description: "A test tool"}
	reg.Add(tool)

	got := reg.Get("test_tool")
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Definition().Name != "test_tool" {
		t.Errorf("name = %q, want 'test_tool'", got.Definition().Name)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	reg := NewRegistry()
	got := reg.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil for missing tool")
	}
}

func TestRegistry_Has(t *testing.T) {
	reg := NewRegistry()
	tool := &mockTool{name: "test_tool"}
	reg.Add(tool)

	if !reg.Has("test_tool") {
		t.Error("Has returned false for existing tool")
	}
	if reg.Has("nonexistent") {
		t.Error("Has returned true for missing tool")
	}
}

func TestRegistry_ToSpecs(t *testing.T) {
	reg := NewRegistry()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file": map[string]any{"type": "string"},
		},
	}
	tool := &mockTool{
		name:        "read_file",
		description: "Read a file",
		parameters:  schema,
	}
	reg.Add(tool)

	specs := reg.ToSpecs()
	if len(specs) != 1 {
		t.Fatalf("specs len = %d, want 1", len(specs))
	}
	if specs[0].Type != "function" {
		t.Errorf("spec type = %q, want 'function'", specs[0].Type)
	}
	if specs[0].Function.Name != "read_file" {
		t.Errorf("spec name = %q, want 'read_file'", specs[0].Function.Name)
	}
}

func TestRegistry_All(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&mockTool{name: "tool_a"})
	reg.Add(&mockTool{name: "tool_b"})

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("All len = %d, want 2", len(all))
	}
}
