package types

import (
	"fmt"
	"strings"
	"time"
)

// MessageType identifies the kind of message.
type MessageType string

const (
	MsgUser       MessageType = "user"
	MsgAssistant  MessageType = "assistant"
	MsgSystem     MessageType = "system"
	MsgToolCall   MessageType = "tool_call"
	MsgToolResult MessageType = "tool_result"
)

// Message represents a single conversation entry.
type Message struct {
	ID           string
	Type         MessageType
	Content      string // Full content (what the LLM sees)
	Summary      string // One-line display summary for tool results
	ThinkingText string
	ToolName     string
	ToolArgs     string
	IsError      bool
	Streaming    bool
	Collapsed        bool // When true, show Summary instead of Content
	ThinkingExpanded bool // When true, show full thinking text
	Timestamp        time.Time
}

// NewMessage creates a new Message with a unique ID.
func NewMessage(mType MessageType, content string) *Message {
	return &Message{
		ID:        fmt.Sprintf("%d-%s", time.Now().UnixNano(), mType),
		Type:      mType,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewSystemMessage creates a system-level message (e.g. cancellation, compaction).
func NewSystemMessage(content string) *Message {
	return &Message{
		ID:        fmt.Sprintf("%d-system", time.Now().UnixNano()),
		Type:      MsgSystem,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewToolCall creates a tool call message.
func NewToolCall(toolName, args string) *Message {
	return &Message{
		ID:        fmt.Sprintf("%d-tool_call", time.Now().UnixNano()),
		Type:      MsgToolCall,
		ToolName:  toolName,
		ToolArgs:  args,
		Timestamp: time.Now(),
	}
}

// NewToolResult creates a tool result message, collapsed by default.
func NewToolResult(toolName, result string, isError bool) *Message {
	return &Message{
		ID:        fmt.Sprintf("%d-tool_result", time.Now().UnixNano()),
		Type:      MsgToolResult,
		Content:   result,
		ToolName:  toolName,
		IsError:   isError,
		Collapsed: true,
		Summary:   generateSummary(toolName, result, isError),
		Timestamp: time.Now(),
	}
}

// generateSummary creates a one-line summary for a tool result.
func generateSummary(toolName, result string, isError bool) string {
	if isError {
		// For errors, show the error message (truncated)
		msg := truncateLine(result, 100)
		return fmt.Sprintf("✗ %s: %s", toolName, msg)
	}

	// For read tool: show file info
	if toolName == "read" {
		lines := strings.Count(result, "\n") + 1
		// Try to extract filename from the result (it's in the ``` block)
		return fmt.Sprintf("✓ read: %d lines", lines)
	}

	// For write tool: show confirmation
	if toolName == "write" {
		return fmt.Sprintf("✓ %s", result)
	}

	// For edit tool: show diff summary
	if toolName == "edit" {
		return fmt.Sprintf("✓ %s edited", toolName)
	}

	// For bash tool: show first line of output
	if toolName == "bash" {
		firstLine := ""
		if idx := strings.Index(result, "\n"); idx >= 0 {
			firstLine = result[:idx]
		} else {
			firstLine = result
		}
		return fmt.Sprintf("✓ bash: %s", truncateLine(firstLine, 80))
	}

	// Generic fallback
	return fmt.Sprintf("✓ %s: %s", toolName, truncateLine(result, 80))
}

func truncateLine(s string, maxLen int) string {
	// Get first line
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
