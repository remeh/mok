package compaction

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/user/mok/internal/llm"
)

// FileOperations tracks file operations from the conversation.
type FileOperations struct {
	ReadFiles    []string `json:"read_files"`
	WrittenFiles []string `json:"written_files"`
	EditedFiles  []string `json:"edited_files"`
}

// ExtractFileOps scans assistant messages for tool calls and extracts file paths.
func ExtractFileOps(messages []llm.Message) FileOperations {
	var ops FileOperations

	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}

		// Check for tool calls in the message
		for _, tc := range msg.ToolCalls {
			toolName := tc.Function.Name
			args := tc.Function.Arguments

			switch toolName {
			case "read":
				if path := extractPathFromArgs(args); path != "" {
					ops.ReadFiles = append(ops.ReadFiles, path)
				}
			case "write":
				if path := extractPathFromArgs(args); path != "" {
					ops.WrittenFiles = append(ops.WrittenFiles, path)
				}
			case "edit":
				if path := extractPathFromArgs(args); path != "" {
					ops.EditedFiles = append(ops.EditedFiles, path)
				}
			}
		}
	}

	// Remove duplicates
	ops.ReadFiles = uniqueStrings(ops.ReadFiles)
	ops.WrittenFiles = uniqueStrings(ops.WrittenFiles)
	ops.EditedFiles = uniqueStrings(ops.EditedFiles)

	return ops
}

// extractPathFromArgs extracts the path field from tool call arguments.
func extractPathFromArgs(args string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return ""
	}

	if path, ok := m["path"].(string); ok {
		return path
	}

	return ""
}

// uniqueStrings removes duplicate strings from a slice.
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// ExtractKeyPoints scans messages for important information to include in the summary.
type KeyPoints struct {
	Goal         string
	Context      string
	CurrentState string
	NextSteps    string
}

// ExtractKeyPoints extracts key information from the conversation.
func ExtractKeyPoints(messages []llm.Message) KeyPoints {
	var points KeyPoints

	// Find the user's initial request (goal)
	for _, msg := range messages {
		if msg.Role == "user" {
			content := strings.TrimSpace(msg.Content)
			if len(content) > 0 {
				points.Goal = content
				break
			}
		}
	}

	// Extract context from conversation
	var contextParts []string
	for _, msg := range messages {
		if msg.Role == "user" || msg.Role == "assistant" {
			content := strings.TrimSpace(msg.Content)
			if len(content) > 100 { // Only include substantial messages
				contextParts = append(contextParts, content)
			}
		}
	}

	if len(contextParts) > 0 {
		// Take last few messages as context
		start := 0
		if len(contextParts) > 5 {
			start = len(contextParts) - 5
		}
		points.Context = strings.Join(contextParts[start:], "\n\n")
	}

	// Get current state from the last assistant message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			content := strings.TrimSpace(messages[i].Content)
			if len(content) > 0 {
				points.CurrentState = content
				break
			}
		}
	}

	// Infer next steps from the last user message before the end
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			content := strings.TrimSpace(messages[i].Content)
			if len(content) > 0 {
				points.NextSteps = content
				break
			}
		}
	}

	return points
}

// Pattern to extract file paths from common tool call argument formats
var pathPattern = regexp.MustCompile(`["']path["']\s*:\s*["']([^"']+)["']`)

// ExtractPathsFromContent extracts file paths mentioned in message content.
func ExtractPathsFromContent(content string) []string {
	matches := pathPattern.FindAllStringSubmatch(content, -1)
	var paths []string
	for _, match := range matches {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}
	return paths
}
