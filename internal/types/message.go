package types

import (
	"fmt"
	"time"
)

// MessageType identifies the kind of message.
type MessageType string

const (
	MsgSystem    MessageType = "system"
	MsgUser      MessageType = "user"
	MsgAssistant MessageType = "assistant"
	MsgToolCall  MessageType = "tool_call"
	MsgToolResult MessageType = "tool_result"
)

// Message represents a single conversation entry.
type Message struct {
	ID           string
	Type         MessageType
	Content      string
	ThinkingText string // Collapsed reasoning/thinking output
	ToolName     string
	ToolArgs     string
	IsError      bool
	Streaming    bool // True while the message is still being streamed
	Timestamp    time.Time
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

// NewToolResult creates a tool result message.
func NewToolResult(toolName, result string, isError bool) *Message {
	return &Message{
		ID:        fmt.Sprintf("%d-tool_result", time.Now().UnixNano()),
		Type:      MsgToolResult,
		Content:   result,
		ToolName:  toolName,
		IsError:   isError,
		Timestamp: time.Now(),
	}
}
