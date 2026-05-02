package tools

import (
	"bytes"
	"encoding/json"
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
- The command runs to completion before returning. For interactive commands, this will hang until timeout.`,
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
		if err != nil {
			// Command exited with error
			output := truncateOutput(stdout.String())
			errOutput := truncateOutput(stderr.String())

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

			return sb.String(), fmt.Errorf("exit code %d", cmd.ProcessState.ExitCode())
		}

		// Success
		output := truncateOutput(stdout.String())
		errOutput := truncateOutput(stderr.String())

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

		return sb.String(), nil

	case <-time.After(timeout):
		cmd.Process.Kill()
		<-done
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
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
