package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// diffColor codes for terminal output
const (
	diffHeaderColor   = "\033[1;36m" // Bright cyan for @@ headers
	diffOldColor      = "\033[31m"   // Red for deleted lines
	diffNewColor      = "\033[32m"   // Green for added lines
	diffContextColor  = "\033[90m"   // Dim gray for context lines
	diffResetColor    = "\033[0m"    // Reset
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
	unifiedDiff := generateCompactDiff(originalStr, modified, resolved)

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
func generateCompactDiff(oldContent, newContent, path string) string {
	var buf strings.Builder

	// Split into lines
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Generate diff using go-diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)

	// Track line numbers
	oldLineNum := 1
	newLineNum := 1

	// Find all change regions
	type changeRegion struct {
		oldStart, oldEnd int // line numbers (1-indexed)
		newStart, newEnd int
		hasOld           bool
		hasNew           bool
	}

	var regions []changeRegion
	var currentRegion *changeRegion

	for _, d := range diffs {
		lineCount := strings.Count(d.Text, "\n")
		if d.Type == diffmatchpatch.DiffEqual {
			// If we have an active region, end it
			if currentRegion != nil {
				currentRegion.oldEnd = oldLineNum - 1
				currentRegion.newEnd = newLineNum - 1
				regions = append(regions, *currentRegion)
				currentRegion = nil
			}
			oldLineNum += lineCount + 1
			newLineNum += lineCount + 1
		} else {
			// Start or continue a change region
			if currentRegion == nil {
				currentRegion = &changeRegion{
					oldStart: oldLineNum,
					newStart: newLineNum,
				}
			}

			if d.Type == diffmatchpatch.DiffDelete {
				currentRegion.hasOld = true
				oldLineNum += lineCount + 1
				currentRegion.oldEnd = oldLineNum - 1
			} else if d.Type == diffmatchpatch.DiffInsert {
				currentRegion.hasNew = true
				newLineNum += lineCount + 1
				currentRegion.newEnd = newLineNum - 1
			}
		}
	}

	// Don't forget the last region
	if currentRegion != nil {
		if !currentRegion.hasOld {
			currentRegion.oldEnd = oldLineNum - 1
		}
		if !currentRegion.hasNew {
			currentRegion.newEnd = newLineNum - 1
		}
		regions = append(regions, *currentRegion)
	}

	// Merge overlapping or adjacent hunks
	const contextLines = 3
	type hunk struct {
		ctxOldStart, ctxOldEnd int // context range in old file
		ctxNewStart, ctxNewEnd int // context range in new file
		regions                []changeRegion
	}

	var hunks []hunk
	var currentHunk *hunk

	for _, r := range regions {
		// Calculate context range for this region
		ctxOldStart := r.oldStart - contextLines
		if ctxOldStart < 1 {
			ctxOldStart = 1
		}
		ctxOldEnd := r.oldEnd + contextLines
		if ctxOldEnd > len(oldLines) {
			ctxOldEnd = len(oldLines)
		}

		// Calculate new context range
		ctxNewStart := r.newStart - contextLines
		if ctxNewStart < 1 {
			ctxNewStart = 1
		}
		ctxNewEnd := r.newEnd + contextLines
		if ctxNewEnd > len(newLines) {
			ctxNewEnd = len(newLines)
		}

		// Check if this region should merge with current hunk
		shouldMerge := false
		if currentHunk != nil {
			// Merge if context ranges overlap or are adjacent (gap of 0 or 1 line)
			if ctxOldStart <= currentHunk.ctxOldEnd+1 {
				shouldMerge = true
			}
		}

		if shouldMerge {
			// Extend current hunk
			if ctxOldStart < currentHunk.ctxOldStart {
				currentHunk.ctxOldStart = ctxOldStart
			}
			if ctxOldEnd > currentHunk.ctxOldEnd {
				currentHunk.ctxOldEnd = ctxOldEnd
			}
			if ctxNewStart < currentHunk.ctxNewStart {
				currentHunk.ctxNewStart = ctxNewStart
			}
			if ctxNewEnd > currentHunk.ctxNewEnd {
				currentHunk.ctxNewEnd = ctxNewEnd
			}
			currentHunk.regions = append(currentHunk.regions, r)
		} else {
			// Start new hunk
			currentHunk = &hunk{
				ctxOldStart:  ctxOldStart,
				ctxOldEnd:    ctxOldEnd,
				ctxNewStart:  ctxNewStart,
				ctxNewEnd:    ctxNewEnd,
				regions:      []changeRegion{r},
			}
			hunks = append(hunks, *currentHunk)
		}
	}

	// Output merged hunks
	for i, h := range hunks {
		if i > 0 {
			buf.WriteString("\n")
		}

		// Build hunk header with merged context range
		oldRange := formatRange(h.ctxOldStart, h.ctxOldEnd)
		newRange := formatRange(h.ctxNewStart, h.ctxNewEnd)
		buf.WriteString(fmt.Sprintf("%s@@ -%s +%s @@%s\n", diffHeaderColor, oldRange, newRange, diffResetColor))

		// Track what we've output to avoid duplicates
		outputOldLine := h.ctxOldStart
		outputNewLine := h.ctxNewStart

		for _, r := range h.regions {
			// Output context lines before this region's changes
			for j := outputOldLine; j < r.oldStart && j <= len(oldLines); j++ {
				buf.WriteString(fmt.Sprintf("%s %s%s\n", diffContextColor, oldLines[j-1], diffResetColor))
				outputOldLine++
				outputNewLine++
			}

			// Output deleted lines
			for j := r.oldStart; j <= r.oldEnd && j <= len(oldLines); j++ {
				buf.WriteString(fmt.Sprintf("%s-%s%s\n", diffOldColor, oldLines[j-1], diffResetColor))
				outputOldLine++
			}

			// Output added lines
			for j := r.newStart; j <= r.newEnd && j <= len(newLines); j++ {
				buf.WriteString(fmt.Sprintf("%s+%s%s\n", diffNewColor, newLines[j-1], diffResetColor))
				outputNewLine++
			}
		}

		// Output remaining context lines after the last change
		for j := outputOldLine; j <= h.ctxOldEnd && j <= len(oldLines); j++ {
			buf.WriteString(fmt.Sprintf("%s %s%s\n", diffContextColor, oldLines[j-1], diffResetColor))
		}
	}

	// Add file headers
	header := fmt.Sprintf("%s--- a/%s%s\n%s+++ b/%s%s\n", diffHeaderColor, path, diffResetColor, diffHeaderColor, path, diffResetColor)
	return header + buf.String()
}

// formatRange formats a line range for hunk headers.
func formatRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, end-start+1)
}
