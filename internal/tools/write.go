package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteArgs represents the write tool arguments.
type WriteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteTool writes content to a file.
type WriteTool struct {
	CWD string
}

// Definition returns the tool's metadata.
func (t *WriteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "write",
		Description: `Create a new file with the given content, or completely overwrite an existing file.

Usage notes:
- Creates parent directories automatically if they don't exist.
- This tool overwrites the entire file. To modify specific parts of an existing file, use the edit tool instead.
- Only use this tool for creating new files or for complete rewrites where most of the content changes.
- Always provide the complete desired file content — partial content will result in a partial file.`,
		Snippet: "Create a new file or completely overwrite an existing one",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write (relative or absolute)",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": false,
		},
	}
}

// Execute writes the content to the file.
func (t *WriteTool) Execute(args json.RawMessage) (string, error) {
	var writeArgs WriteArgs
	if err := json.Unmarshal(args, &writeArgs); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if writeArgs.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if writeArgs.Content == "" {
		return "", fmt.Errorf("content is required")
	}

	resolved, err := ResolvePath(writeArgs.Path, t.CWD)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Create parent directories if needed
	dir := filepath.Dir(resolved)
	if !DirExists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directories: %w", err)
		}
	}

	// Write the file
	if err := os.WriteFile(resolved, []byte(writeArgs.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(writeArgs.Content), resolved), nil
}
