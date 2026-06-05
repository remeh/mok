package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	maxBashOutputLines = 2000
	maxBashOutputBytes = 50 * 1024 // 50KB
	defaultBashTimeout = 30 * time.Second
)

// BashArgs represents the bash tool arguments.
type BashArgs struct {
	Command string `json:"command"`
	Timeout *int   `json:"timeout,omitempty"` // seconds
}

// BashTool executes shell commands.
type BashTool struct {
	CWD     string
	Timeout time.Duration
}

// NeedsConfirmation returns true if the command matches a dangerous pattern
// based on the configured policy. Uses two-pass matching for allowlist mode:
//
//  1. If policy is "allowlist": check if command starts with any safe prefix.
//     If not in allowlist → confirm.
//  2. Always scan for dangerous parameter patterns (substring match) regardless
//     of allowlist status — handles cases like `find /tmp -delete`.
func (t BashTool) NeedsConfirmation(command string, policy string, blocklist []string, allowlist []string) bool {
	switch policy {
	case "none":
		return false
	case "allowlist":
		if !matchesAnyPrefix(command, allowlist) {
			return true // not in allowlist → confirm
		}
		// In allowlist but still check dangerous parameter patterns
		return matchesAnySubstring(command, blocklist)
	default: // "blocklist", "", or any unrecognized policy
		return matchesAnySubstring(command, blocklist)
	}
}

// matchesAnyPrefix checks if the command starts with any of the given prefixes (case-insensitive).
func matchesAnyPrefix(command string, patterns []string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	for _, p := range patterns {
		if strings.HasPrefix(cmd, strings.ToLower(strings.TrimSpace(p))) {
			return true
		}
	}
	return false
}

// matchesAnySubstring checks if the command contains any of the given substrings (case-insensitive).
func matchesAnySubstring(command string, patterns []string) bool {
	cmd := strings.ToLower(command)
	for _, p := range patterns {
		if strings.Contains(cmd, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// Definition returns the tool's metadata.
func (t *BashTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "bash",
		Description: `Execute a shell command and return its output (stdout and stderr combined).

Usage notes:
- Commands run in a subshell (bash) in the working directory with a default timeout of 30 seconds.
- Use the timeout parameter for long-running commands (e.g., builds, tests).
- Use for: running builds, tests, git commands, file searches (ls, grep, find), installing packages, and other shell operations.
- Do NOT use for reading file contents (use the read tool) or editing files (use the edit tool).
- Output is truncated to 2000 lines / 50KB. For commands with large output, pipe through head/tail/grep.
- The command runs to completion before returning. For interactive commands, this will hang until timeout.
- Exit codes are included in the output (e.g., [exit code: 1]). A non-zero exit code is not always a failure — for example, grep returns 1 when no matches are found, diff returns 1 when files differ, and test/[ ] returns 1 for false conditions. Review the output and exit code together to determine the actual outcome.

Example call: {"command": "git status", "timeout": 10}`,
		Snippet: "Run a shell command with stdout/stderr capture",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to execute",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Timeout in seconds (default: 30)",
				},
			},
			"required":             []string{"command"},
			"additionalProperties": false,
		},
	}
}

// Execute runs the command and returns the output.
func (t *BashTool) Execute(args json.RawMessage) (string, error) {
	var bashArgs BashArgs
	if err := json.Unmarshal(args, &bashArgs); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if bashArgs.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Determine timeout
	timeout := t.Timeout
	if timeout == 0 {
		timeout = defaultBashTimeout
	}
	if bashArgs.Timeout != nil && *bashArgs.Timeout > 0 {
		timeout = time.Duration(*bashArgs.Timeout) * time.Second
	}

	// Choose shell based on OS
	shell := "sh"
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		shell = "bash"
	}

	// Create command
	cmd := exec.Command(shell, "-c", bashArgs.Command)
	cmd.Dir = t.CWD

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Wait with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		output := buildOutput(stdout.String(), stderr.String())
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// Command ran but exited with a non-zero code. This is normal
				// for many tools (e.g. grep with no matches = 1, diff with
				// differences = 1). Include the exit code so the LLM can
				// reason about it, but do not return an error.
				exitCode := exitErr.ExitCode()
				if output != "" {
					output += fmt.Sprintf("\n[exit code: %d]", exitCode)
				} else {
					output = fmt.Sprintf("[exit code: %d]", exitCode)
				}
				return output, nil
			}
			// Command failed to run (e.g. binary not found, permission denied to start)
			return output, fmt.Errorf("failed to run command: %w", err)
		}
		return output, nil

	case <-time.After(timeout):
		cmd.Process.Kill()
		<-done
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
}

func buildOutput(stdout, stderr string) string {
	output := truncateOutput(stdout)
	errOutput := truncateOutput(stderr)

	var sb strings.Builder
	if output != "" {
		sb.WriteString(output)
	}
	if errOutput != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(errOutput)
	}
	return sb.String()
}

func truncateOutput(output string) string {
	// Check byte limit
	if len(output) > maxBashOutputBytes {
		output = output[:maxBashOutputBytes]
		// Find the last newline to avoid cutting mid-line
		lastNewline := strings.LastIndex(output, "\n")
		if lastNewline > 0 {
			output = output[:lastNewline]
		}
		output += "\n... (output truncated)"
		return output
	}

	// Check line limit
	lines := strings.Split(output, "\n")
	if len(lines) > maxBashOutputLines {
		lines = lines[:maxBashOutputLines]
		output = strings.Join(lines, "\n")
		output += "\n... (output truncated)"
	}

	return output
}
