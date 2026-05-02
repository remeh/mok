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
	Path  string   `json:"path"`
	Edits []EditOp `json:"edits"`
}

// EditOp represents a single search/replace edit.
type EditOp struct {
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
		Name: "edit",
		Description: `Edit a file by performing one or more search/replace operations. Returns a unified diff of the changes.

Usage notes:
- You MUST read the file first before editing. Use the read tool to see the current contents.
- oldText must match the file content exactly, including whitespace and indentation. Copy the exact text from the read output.
- Multiple edits in one call are all matched against the original file content (not incrementally against prior edits in the same call).
- If oldText is not found, the edit fails. Re-read the file with the read tool to see its actual content before retrying.
- Use this tool instead of bash sed/awk — it validates changes and shows a diff of what changed.
- Prefer this tool over write for modifying existing files, as it only changes what needs to change.

Example call: {"path": "src/main.go", "edits": [{"oldText": "fmt.Println(\"hello\")", "newText": "fmt.Println(\"hello world\")"}]}`,
		Snippet: "Search/replace edit with exact matching and diff output",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit (relative or absolute)",
				},
				"edits": map[string]any{
					"type":        "array",
					"description": "List of edit operations to apply",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"oldText": map[string]any{
								"type":        "string",
								"description": "Exact text to find (must match including whitespace)",
							},
							"newText": map[string]any{
								"type":        "string",
								"description": "Replacement text",
							},
						},
						"required": []string{"oldText", "newText"},
					},
				},
			},
			"required":             []string{"path", "edits"},
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
	if len(editArgs.Edits) == 0 {
		return "", fmt.Errorf("edits list is required and must not be empty")
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
	modified := originalStr

	// Track all edits for reporting
	var appliedEdits []editReport
	var failedEdits []editError

	for i, edit := range editArgs.Edits {
		if edit.OldText == "" {
			failedEdits = append(failedEdits, editError{
				index: i,
				err:   "oldText must not be empty",
			})
			continue
		}

		// Find in original content
		idx := strings.Index(modified, edit.OldText)
		if idx == -1 {
			failedEdits = append(failedEdits, editError{
				index: i,
				err:   fmt.Sprintf("text not found: %q", truncateForError(edit.OldText)),
			})
			continue
		}

		// Apply edit
		before := modified[:idx]
		after := modified[idx+len(edit.OldText):]
		modified = before + edit.NewText + after

		appliedEdits = append(appliedEdits, editReport{
			index:     i,
			oldLength: len(edit.OldText),
			newLength: len(edit.NewText),
		})
	}

	// If no edits were applied, return error
	if len(appliedEdits) == 0 {
		errMsgs := make([]string, 0, len(failedEdits))
		for _, e := range failedEdits {
			errMsgs = append(errMsgs, fmt.Sprintf("edit %d: %s", e.index, e.err))
		}
		return "", fmt.Errorf("no edits applied:\n%s", strings.Join(errMsgs, "\n"))
	}

	// Generate unified diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(originalStr, modified, false)
	patches := dmp.PatchMake(diffs)
	unifiedDiff := dmp.PatchToText(patches)

	// Write the modified file
	if err := os.WriteFile(resolved, []byte(modified), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Build result message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Applied %d edit(s) to %s\n\n", len(appliedEdits), resolved))
	sb.WriteString(unifiedDiff)

	if len(failedEdits) > 0 {
		sb.WriteString("\nFailed edits:\n")
		for _, e := range failedEdits {
			sb.WriteString(fmt.Sprintf("  edit %d: %s\n", e.index, e.err))
		}
	}

	return sb.String(), nil
}

type editReport struct {
	index     int
	oldLength int
	newLength int
}

type editError struct {
	index int
	err   string
}

func truncateForError(s string) string {
	if len(s) > 100 {
		return s[:97] + "..."
	}
	return s
}
