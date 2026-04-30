package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupReadTool(t *testing.T) (*ReadTool, string) {
	t.Helper()
	tmpdir := t.TempDir()
	return &ReadTool{CWD: tmpdir}, tmpdir
}

func TestReadToolDefinition(t *testing.T) {
	tool := &ReadTool{}
	def := tool.Definition()

	if def.Name != "read" {
		t.Errorf("name = %q, want 'read'", def.Name)
	}
	if def.Description == "" {
		t.Error("description should not be empty")
	}
	if def.Parameters == nil {
		t.Error("parameters should not be nil")
	}

	// Check required fields
	required, ok := def.Parameters["required"].([]string)
	if !ok {
		t.Fatal("required field not found in parameters")
	}
	if len(required) != 1 || required[0] != "path" {
		t.Errorf("required = %v, want ['path']", required)
	}
}

func TestReadToolExecute_Success(t *testing.T) {
	tool, tmpdir := setupReadTool(t)

	// Create a test file
	content := "line1\nline2\nline3\nline4\nline5"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(ReadArgs{Path: "test.txt"})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "line1") {
		t.Error("result should contain line1")
	}
	if !strings.Contains(result, "line5") {
		t.Error("result should contain line5")
	}
}

func TestReadToolExecute_Offset(t *testing.T) {
	tool, tmpdir := setupReadTool(t)

	content := "line1\nline2\nline3\nline4\nline5"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	offset := 3
	args, _ := json.Marshal(ReadArgs{Path: "test.txt", Offset: &offset})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result, "line1") {
		t.Error("result should not contain line1 with offset 3")
	}
	if !strings.Contains(result, "line3") {
		t.Error("result should contain line3 with offset 3")
	}
}

func TestReadToolExecute_Limit(t *testing.T) {
	tool, tmpdir := setupReadTool(t)

	content := "line1\nline2\nline3\nline4\nline5"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	limit := 2
	args, _ := json.Marshal(ReadArgs{Path: "test.txt", Limit: &limit})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result, "line3") {
		t.Error("result should not contain line3 with limit 2")
	}
	if !strings.Contains(result, "line1") {
		t.Error("result should contain line1 with limit 2")
	}
}

func TestReadToolExecute_NotFound(t *testing.T) {
	tool, _ := setupReadTool(t)

	args, _ := json.Marshal(ReadArgs{Path: "nonexistent.txt"})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadToolExecute_EmptyPath(t *testing.T) {
	tool, _ := setupReadTool(t)

	args, _ := json.Marshal(ReadArgs{Path: ""})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestReadToolExecute_OffsetOutOfRange(t *testing.T) {
	tool, tmpdir := setupReadTool(t)

	content := "line1\nline2"
	path := filepath.Join(tmpdir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	offset := 100
	args, _ := json.Marshal(ReadArgs{Path: "test.txt", Offset: &offset})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "out of range") {
		t.Error("result should indicate out of range")
	}
}
