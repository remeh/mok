package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// EditArgs represents the edit tool arguments.
type EditArgs struct {
	Path    string `json:"path"`
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

// EditTool performs search/replace edits with diff output.
type EditTool struct {
	CWD string
}

// Definition returns the tool's metadata.
func (t *EditTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "edit",
		Description: "Edit a file by performing a search/replace operation. OldText must match exactly including whitespace. For multiple edits, call this tool multiple times.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit (relative or absolute)",
				},
				"oldText": map[string]any{
					"type":        "string",
					"description": "Exact text to find (must match including whitespace)",
				},
				"newText": map[string]any{
					"type":        "string",
					"description": "Replacement text",
				},
			},
			"required":             []string{"path", "oldText", "newText"},
			"additionalProperties": false,
		},
	}
}

// Execute applies the edits to the file.
func (t *EditTool) Execute(args json.RawMessage) (string, error) {
	var editArgs EditArgs
	if err := json.Unmarshal(args, &editArgs); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if editArgs.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if editArgs.OldText == "" {
		return "", fmt.Errorf("oldText must not be empty")
	}

	resolved, err := ResolvePath(editArgs.Path, t.CWD)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	if !FileExists(resolved) {
		return "", fmt.Errorf("file not found: %s", resolved)
	}

	// Read original content
	original, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	originalStr := string(original)

	// Find the old text
	idx := strings.Index(originalStr, editArgs.OldText)
	if idx == -1 {
		return "", fmt.Errorf("text not found: %q", truncateForError(editArgs.OldText))
	}

	// Apply edit
	modified := originalStr[:idx] + editArgs.NewText + originalStr[idx+len(editArgs.OldText):]

	// Generate unified diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(originalStr, modified, false)
	patches := dmp.PatchMake(diffs)
	unifiedDiff := dmp.PatchToText(patches)

	// Write the modified file
	if err := os.WriteFile(resolved, []byte(modified), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Applied edit to %s\n\n%s", resolved, unifiedDiff), nil
}

func truncateForError(s string) string {
	if len(s) > 100 {
		return s[:97] + "..."
	}
	return s
}
