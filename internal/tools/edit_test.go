package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupEditTool(t *testing.T) (*EditTool, string) {
	t.Helper()
	tmpdir := t.TempDir()
	return &EditTool{CWD: tmpdir}, tmpdir
}

func TestEditToolDefinition(t *testing.T) {
	tool := &EditTool{}
	def := tool.Definition()

	if def.Name != "edit" {
		t.Errorf("name = %q, want 'edit'", def.Name)
	}
	if def.Description == "" {
		t.Error("description should not be empty")
	}

	required, ok := def.Parameters["required"].([]string)
	if !ok {
		t.Fatal("required field not found in parameters")
	}
	if len(required) != 2 {
		t.Errorf("required count = %d, want 2", len(required))
	}
}

func TestEditToolExecute_Success(t *testing.T) {
	tool, tmpdir := setupEditTool(t)

	content := "Hello, world!\nThis is a test."
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(EditArgs{
		Path: "test.txt",
		Edits: []EditOp{
			{OldText: "world", NewText: "universe"},
		},
	})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "@@") {
		t.Error("result should contain diff markers")
	}

	// Verify file was edited
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read edited file: %v", err)
	}
	if !strings.Contains(string(data), "universe") {
		t.Errorf("file should contain 'universe', got: %s", string(data))
	}
	if strings.Contains(string(data), "world") {
		t.Errorf("file should not contain 'world', got: %s", string(data))
	}
}

func TestEditToolExecute_MultipleEdits(t *testing.T) {
	tool, tmpdir := setupEditTool(t)

	content := "foo bar baz"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(EditArgs{
		Path: "test.txt",
		Edits: []EditOp{
			{OldText: "foo", NewText: "FOO"},
			{OldText: "bar", NewText: "BAR"},
			{OldText: "baz", NewText: "BAZ"},
		},
	})
	_, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all edits were applied
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read edited file: %v", err)
	}
	expected := "FOO BAR BAZ"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditToolExecute_NotFound(t *testing.T) {
	tool, tmpdir := setupEditTool(t)

	content := "Hello, world!"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(EditArgs{
		Path: "test.txt",
		Edits: []EditOp{
			{OldText: "nonexistent", NewText: "replacement"},
		},
	})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for text not found")
	}
}

func TestEditToolExecute_EmptyPath(t *testing.T) {
	tool, _ := setupEditTool(t)

	args, _ := json.Marshal(EditArgs{Path: "", Edits: []EditOp{{OldText: "a", NewText: "b"}}})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestEditToolExecute_EmptyEdits(t *testing.T) {
	tool, tmpdir := setupEditTool(t)

	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(EditArgs{Path: "test.txt", Edits: []EditOp{}})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty edits")
	}
}

func TestEditToolExecute_FileNotFound(t *testing.T) {
	tool, _ := setupEditTool(t)

	args, _ := json.Marshal(EditArgs{
		Path: "nonexistent.txt",
		Edits: []EditOp{{OldText: "a", NewText: "b"}},
	})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestEditToolExecute_AllAgainstOriginal(t *testing.T) {
	tool, tmpdir := setupEditTool(t)

	content := "foo foo foo"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// First edit changes first "foo" to "FOO"
	// Second edit should match against original, so it changes the second "foo" to "BAR"
	// But since we apply edits sequentially on modified content, this test validates behavior
	args, _ := json.Marshal(EditArgs{
		Path: "test.txt",
		Edits: []EditOp{
			{OldText: "foo", NewText: "FOO"},
			{OldText: "foo", NewText: "BAR"},
		},
	})
	_, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read edited file: %v", err)
	}
	// After applying edits sequentially: "FOO BAR foo"
	expected := "FOO BAR foo"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}
