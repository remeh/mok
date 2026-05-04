package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupBashTool(t *testing.T) (*BashTool, string) {
	t.Helper()
	tmpdir := t.TempDir()
	return &BashTool{CWD: tmpdir, Timeout: 5 * time.Second}, tmpdir
}

func TestBashToolDefinition(t *testing.T) {
	tool := &BashTool{}
	def := tool.Definition()

	if def.Name != "bash" {
		t.Errorf("name = %q, want 'bash'", def.Name)
	}
	if def.Description == "" {
		t.Error("description should not be empty")
	}

	required, ok := def.Parameters["required"].([]string)
	if !ok {
		t.Fatal("required field not found in parameters")
	}
	if len(required) != 1 || required[0] != "command" {
		t.Errorf("required = %v, want ['command']", required)
	}
}

func TestBashToolExecute_Success(t *testing.T) {
	tool, _ := setupBashTool(t)

	args, _ := json.Marshal(BashArgs{Command: "echo hello"})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "hello") {
		t.Errorf("result should contain 'hello', got: %s", result)
	}
}

func TestBashToolExecute_WithStderr(t *testing.T) {
	tool, _ := setupBashTool(t)

	args, _ := json.Marshal(BashArgs{Command: "echo stdout; echo stderr >&2"})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "stdout") {
		t.Error("result should contain stdout")
	}
	if !strings.Contains(result, "stderr") {
		t.Error("result should contain stderr")
	}
}

func TestBashToolExecute_NonZeroExitCode(t *testing.T) {
	tool, _ := setupBashTool(t)

	// Exit code 1 with output - includes exit code in output, does NOT return error
	args, _ := json.Marshal(BashArgs{Command: "echo some output; exit 1"})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("non-zero exit should not return error, got: %v", err)
	}
	if !strings.Contains(result, "some output") {
		t.Errorf("result should contain output, got: %s", result)
	}
	if !strings.Contains(result, "[exit code: 1]") {
		t.Errorf("result should contain exit code, got: %s", result)
	}

	// Empty output with non-zero exit
	args, _ = json.Marshal(BashArgs{Command: "exit 42"})
	result, err = tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("non-zero exit should not return error, got: %v", err)
	}
	if result != "[exit code: 42]" {
		t.Errorf("result should just be exit code, got: %s", result)
	}

	// Grep with no matches returns exit code 1 - this is information, not failure
	args, _ = json.Marshal(BashArgs{Command: "echo 'line one\nline two' | grep 'three'"})
	result, err = tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("grep exit code 1 should not return error, got: %v", err)
	}
	if !strings.Contains(result, "[exit code: 1]") {
		t.Errorf("grep no-match should report exit code 1, got: %s", result)
	}

	// diff with differences returns exit code 1
	args, _ = json.Marshal(BashArgs{Command: "diff <(echo a) <(echo b)"})
	result, err = tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("diff exit code 1 should not return error, got: %v", err)
	}
	if !strings.Contains(result, "[exit code: 1]") {
		t.Errorf("diff with differences should report exit code 1, got: %s", result)
	}
}

func TestBashToolExecute_InvalidCommand(t *testing.T) {
	tool, _ := setupBashTool(t)

	// Unknown binary - returns output with exit code 127, not an error.
	// The LLM gets the shell's "command not found" message so it can reason about it.
	args, _ := json.Marshal(BashArgs{Command: "doesnotexistquux 2>&1"})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("command-not-found should not return error, got: %v", err)
	}
	if !strings.Contains(result, "[exit code: 127]") && !strings.Contains(result, "not found") {
		t.Errorf("result should contain exit code 127 or 'not found', got: %s", result)
	}
}

func TestBashToolExecute_CWD(t *testing.T) {
	tool, tmpdir := setupBashTool(t)

	// Create a file in tmpdir
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	args, _ := json.Marshal(BashArgs{Command: "ls test.txt"})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "test.txt") {
		t.Errorf("result should contain 'test.txt', got: %s", result)
	}
}

func TestBashToolExecute_EmptyCommand(t *testing.T) {
	tool, _ := setupBashTool(t)

	args, _ := json.Marshal(BashArgs{Command: ""})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestBashToolExecute_CustomTimeout(t *testing.T) {
	tool, _ := setupBashTool(t)

	timeout := 1
	args, _ := json.Marshal(BashArgs{Command: "sleep 10", Timeout: &timeout})
	_, err := tool.Execute(json.RawMessage(args))
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should mention timeout, got: %v", err)
	}
}

func TestBashToolExecute_OutputTruncation(t *testing.T) {
	tool, _ := setupBashTool(t)

	// Generate a large output
	cmd := "for i in $(seq 1 3000); do echo \"line $i\"; done"
	args, _ := json.Marshal(BashArgs{Command: cmd})
	result, err := tool.Execute(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "truncated") {
		t.Error("result should indicate truncation for large output")
	}
}

func TestBashToolExecute_InvalidJSON(t *testing.T) {
	tool, _ := setupBashTool(t)

	_, err := tool.Execute(json.RawMessage(`{invalid json}`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
