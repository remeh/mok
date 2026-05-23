package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
		Name: "edit",
		Description: `Edit a file by performing a single search/replace operation. Returns a unified diff of the changes.

Usage notes:
- You MUST read the file first before editing. Use the read tool to see the current contents.
- oldText must match the file content exactly, including whitespace and indentation. Copy the exact text from the read output.
- If oldText is not found, the edit fails. Re-read the file with the read tool to see its actual content before retrying.
- Use this tool instead of bash sed/awk — it validates changes and shows a diff of what changed.
- Prefer this tool over write for modifying existing files, as it only changes what needs to change.
- For multiple changes to the same file, call this tool multiple times.

Example call: {"path": "src/main.go", "oldText": "fmt.Println(\"hello\")", "newText": "fmt.Println(\"hello world\")"}`,
		Snippet: "Search/replace edit with exact matching and diff output",
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

// Execute applies the edit to the file.
func (t *EditTool) Execute(args json.RawMessage) (string, error) {
	var editArgs EditArgs
	if err := json.Unmarshal(args, &editArgs); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if editArgs.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if editArgs.OldText == "" {
		return "", fmt.Errorf("oldText is required and must not be empty")
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

	// Find oldText in content
	idx := strings.Index(originalStr, editArgs.OldText)
	if idx == -1 {
		return "", fmt.Errorf("text not found: %q", truncateForError(editArgs.OldText))
	}

	// Apply edit
	modified := originalStr[:idx] + editArgs.NewText + originalStr[idx+len(editArgs.OldText):]

	// Generate compact unified diff with context
	unifiedDiff := generateCompactDiff(originalStr, modified, resolved, idx, len(editArgs.OldText), len(editArgs.NewText))

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

// generateCompactDiff creates a compact unified diff showing only
// changed lines with minimal context (3 lines before/after).
//
// Since edits are a single contiguous search/replace at a known byte offset,
// we derive the affected line range directly from the offset instead of running
// a generic diff algorithm — this avoids miscounting line numbers on sub-line
// edits (e.g. replacing a word inside a line).
func generateCompactDiff(oldContent, newContent, path string, idx, oldLen, newLen int) string {
	const contextLines = 3

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Expand the byte range to whole-line boundaries in the old content.
	oldStartByte := idx
	for oldStartByte > 0 && oldContent[oldStartByte-1] != '\n' {
		oldStartByte--
	}
	oldEndByte := idx + oldLen
	for oldEndByte < len(oldContent) && oldContent[oldEndByte] != '\n' {
		oldEndByte++
	}

	// Same range in the new content (start byte is identical; end shifts by
	// the difference between newText and oldText lengths).
	newStartByte := oldStartByte
	newEndByte := oldEndByte + (newLen - oldLen)

	// 1-indexed line numbers of the changed range.
	oldStartLine := strings.Count(oldContent[:oldStartByte], "\n") + 1
	newStartLine := strings.Count(newContent[:newStartByte], "\n") + 1

	oldChangedCount := strings.Count(oldContent[oldStartByte:oldEndByte], "\n") + 1
	newChangedCount := strings.Count(newContent[newStartByte:newEndByte], "\n") + 1

	oldEndLine := oldStartLine + oldChangedCount - 1
	newEndLine := newStartLine + newChangedCount - 1

	// Surround with up to contextLines on either side.
	ctxOldStart := max(oldStartLine-contextLines, 1)
	ctxOldEnd := min(oldEndLine+contextLines, len(oldLines))
	ctxNewStart := max(newStartLine-contextLines, 1)
	ctxNewEnd := min(newEndLine+contextLines, len(newLines))

	var buf strings.Builder
	fmt.Fprintf(&buf, "--- a/%s\n+++ b/%s\n", path, path)
	fmt.Fprintf(&buf, "@@ -%s +%s @@\n",
		formatRange(ctxOldStart, ctxOldEnd),
		formatRange(ctxNewStart, ctxNewEnd))

	// Leading context.
	for j := ctxOldStart; j < oldStartLine && j <= len(oldLines); j++ {
		fmt.Fprintf(&buf, " %s\n", oldLines[j-1])
	}
	// Deleted lines.
	for j := oldStartLine; j <= oldEndLine && j <= len(oldLines); j++ {
		fmt.Fprintf(&buf, "-%s\n", oldLines[j-1])
	}
	// Added lines.
	for j := newStartLine; j <= newEndLine && j <= len(newLines); j++ {
		fmt.Fprintf(&buf, "+%s\n", newLines[j-1])
	}
	// Trailing context.
	for j := oldEndLine + 1; j <= ctxOldEnd && j <= len(oldLines); j++ {
		fmt.Fprintf(&buf, " %s\n", oldLines[j-1])
	}

	return buf.String()
}

// formatRange formats a line range for hunk headers as "start,count".
func formatRange(start, end int) string {
	count := end - start + 1
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}
