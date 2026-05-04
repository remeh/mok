package types

import (
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage(MsgUser, "hello world")

	if msg.Type != MsgUser {
		t.Errorf("type = %q, want %q", msg.Type, MsgUser)
	}
	if msg.Content != "hello world" {
		t.Errorf("content = %q, want 'hello world'", msg.Content)
	}
	if msg.ID == "" {
		t.Error("ID should not be empty")
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestNewMessageUniqueIDs(t *testing.T) {
	msg1 := NewMessage(MsgUser, "test")
	msg2 := NewMessage(MsgUser, "test")

	if msg1.ID == msg2.ID {
		t.Error("messages should have unique IDs")
	}
}

func TestNewToolCall(t *testing.T) {
	msg := NewToolCall("read", `path="test.go"`)

	if msg.Type != MsgToolCall {
		t.Errorf("type = %q, want %q", msg.Type, MsgToolCall)
	}
	if msg.ToolName != "read" {
		t.Errorf("toolName = %q, want 'read'", msg.ToolName)
	}
	if msg.ToolArgs != `path="test.go"` {
		t.Errorf("toolArgs = %q", msg.ToolArgs)
	}
}

func TestNewToolResult(t *testing.T) {
	msg := NewToolResult("read", "file contents", false)

	if msg.Type != MsgToolResult {
		t.Errorf("type = %q, want %q", msg.Type, MsgToolResult)
	}
	if msg.Content != "file contents" {
		t.Errorf("content = %q", msg.Content)
	}
	if msg.IsError {
		t.Error("isError should be false")
	}
}

func TestNewToolResultError(t *testing.T) {
	msg := NewToolResult("edit", "file not found", true)

	if !msg.IsError {
		t.Error("isError should be true")
	}
}

func TestMessageTypes(t *testing.T) {
	types := []MessageType{
		MsgUser,
		MsgAssistant,
		MsgToolCall,
		MsgToolResult,
	}

	expected := map[MessageType]bool{
		MsgUser:      true,
		MsgAssistant: true,
		MsgToolCall:  true,
		MsgToolResult: true,
	}

	for _, mType := range types {
		if !expected[mType] {
			t.Errorf("unexpected message type: %q", mType)
		}
	}
}

func TestNewMessageTimestamps(t *testing.T) {
	before := time.Now()
	msg := NewMessage(MsgUser, "test")
	after := time.Now()

	if msg.Timestamp.Before(before) || msg.Timestamp.After(after) {
		t.Errorf("timestamp %v not between %v and %v", msg.Timestamp, before, after)
	}
}
