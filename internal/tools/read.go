package tools

import (
	"encoding/json"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxReadLines = 2000
	maxReadBytes = 50 * 1024 // 50KB
)

// ReadArgs represents the read tool arguments.
type ReadArgs struct {
	Path   string `json:"path"`
	Offset *int   `json:"offset,omitempty"`
	Limit  *int   `json:"limit,omitempty"`
}

// ReadTool reads file contents with offset/limit support.
type ReadTool struct {
	CWD string
}

// Definition returns the tool's metadata.
func (t *ReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "read",
		Description: "Read the contents of a text file. For large files, use offset and limit to read in chunks.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read (relative or absolute)",
				},
				"offset": map[string]any{
					"type":        "number",
					"description": "Line number to start reading from (1-indexed)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of lines to read",
				},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	}
}

// Execute reads the file and returns its contents.
func (t *ReadTool) Execute(args json.RawMessage) (string, error) {
	var readArgs ReadArgs
	if err := json.Unmarshal(args, &readArgs); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if readArgs.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := ResolvePath(readArgs.Path, t.CWD)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	if !FileExists(resolved) {
		return "", fmt.Errorf("file not found: %s", resolved)
	}

	// Read as text
	return t.readText(resolved, readArgs.Offset, readArgs.Limit)
}

func (t *ReadTool) readText(path string, offset, limit *int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Apply offset (1-indexed)
	start := 0
	if offset != nil && *offset > 0 {
		start = *offset - 1
		if start >= len(lines) {
			return fmt.Sprintf("File %s has %d lines, offset %d is out of range", path, len(lines), *offset), nil
		}
	}

	// Apply limit
	end := len(lines)
	if limit != nil && *limit > 0 {
		end = start + *limit
		if end > len(lines) {
			end = len(lines)
		}
	}

	// Truncate if too many lines
	if end-start > maxReadLines {
		end = start + maxReadLines
	}

	resultLines := lines[start:end]
	content := strings.Join(resultLines, "\n")

	// Add truncation notice if needed
	var suffix string
	if end < len(lines) {
		suffix = fmt.Sprintf("\n\n... (truncated, %d more lines available from line %d)", len(lines)-end, end+1)
	}

	return fmt.Sprintf("```%s\n%s\n```%s", filepath.Base(path), content, suffix), nil
}