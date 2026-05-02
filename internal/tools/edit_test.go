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
	if len(required) != 3 {
		t.Errorf("required count = %d, want 3", len(required))
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
		Path:    "test.txt",
		OldText: "world",
		NewText: "universe",
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

func TestEditToolExecute_NotFound(t *testing.T) {
	tool, tmpdir := setupEditTool(t)

	content := "Hello, world!"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(EditArgs{
		Path:    "test.txt",
		OldText: "nonexistent",
		NewText: "replacement",
	})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for text not found")
	}
}

func TestEditToolExecute_EmptyPath(t *testing.T) {
	tool, _ := setupEditTool(t)

	args, _ := json.Marshal(EditArgs{Path: "", OldText: "a", NewText: "b"})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestEditToolExecute_EmptyOldText(t *testing.T) {
	tool, tmpdir := setupEditTool(t)

	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(EditArgs{Path: "test.txt", OldText: "", NewText: "x"})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty oldText")
	}
}

func TestEditToolExecute_FileNotFound(t *testing.T) {
	tool, _ := setupEditTool(t)

	args, _ := json.Marshal(EditArgs{
		Path:    "nonexistent.txt",
		OldText: "a",
		NewText: "b",
	})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
