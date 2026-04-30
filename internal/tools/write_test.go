package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupWriteTool(t *testing.T) (*WriteTool, string) {
	t.Helper()
	tmpdir := t.TempDir()
	return &WriteTool{CWD: tmpdir}, tmpdir
}

func TestWriteToolDefinition(t *testing.T) {
	tool := &WriteTool{}
	def := tool.Definition()

	if def.Name != "write" {
		t.Errorf("name = %q, want 'write'", def.Name)
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

func TestWriteToolExecute_Success(t *testing.T) {
	tool, tmpdir := setupWriteTool(t)

	content := "Hello, world!"
	args, _ := json.Marshal(WriteArgs{Path: "test.txt", Content: content})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("result should not be empty")
	}

	// Verify file was written
	path := filepath.Join(tmpdir, "test.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

func TestWriteToolExecute_CreateDirs(t *testing.T) {
	tool, tmpdir := setupWriteTool(t)

	content := "Hello, world!"
	args, _ := json.Marshal(WriteArgs{Path: "subdir/nested/test.txt", Content: content})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("result should not be empty")
	}

	// Verify file was written
	path := filepath.Join(tmpdir, "subdir/nested/test.txt")
	if !FileExists(path) {
		t.Error("file should exist after write")
	}
}

func TestWriteToolExecute_EmptyPath(t *testing.T) {
	tool, _ := setupWriteTool(t)

	args, _ := json.Marshal(WriteArgs{Path: "", Content: "test"})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestWriteToolExecute_EmptyContent(t *testing.T) {
	tool, _ := setupWriteTool(t)

	args, _ := json.Marshal(WriteArgs{Path: "test.txt", Content: ""})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestWriteToolExecute_InvalidJSON(t *testing.T) {
	tool, _ := setupWriteTool(t)

	_, err := tool.Execute(json.RawMessage(`{invalid json}`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
