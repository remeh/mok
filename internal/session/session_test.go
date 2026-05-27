package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/mok/internal/types"
)

func TestNewSession(t *testing.T) {
	input := NewSessionInput{
		Model:               "test-model",
		Endpoint:            "http://localhost:8000/v1",
		CWD:                 "/tmp",
		MaxContextTokens:    131072,
		CompactionThreshold: 0.8,
		KeepRecentTokens:    16384,
		MaxTokens:           4096,
		SummarizationModel:  "summarize-model",
	}

	sess := NewSession(input)

	if sess.Metadata.Version != CurrentVersion {
		t.Errorf("version = %q, want %q", sess.Metadata.Version, CurrentVersion)
	}
	if sess.Metadata.Model != "test-model" {
		t.Errorf("model = %q, want %q", sess.Metadata.Model, "test-model")
	}
	if sess.Metadata.Endpoint != "http://localhost:8000/v1" {
		t.Errorf("endpoint = %q, want %q", sess.Metadata.Endpoint, "http://localhost:8000/v1")
	}
	if sess.Config.MaxContextTokens != 131072 {
		t.Errorf("max_context_tokens = %d, want %d", sess.Config.MaxContextTokens, 131072)
	}
	if sess.Config.CompactionThreshold != 0.8 {
		t.Errorf("compaction_threshold = %f, want %f", sess.Config.CompactionThreshold, 0.8)
	}
	if sess.Config.SummarizationModel != "summarize-model" {
		t.Errorf("summarization_model = %q, want %q", sess.Config.SummarizationModel, "summarize-model")
	}
	if len(sess.Messages) != 0 {
		t.Errorf("messages length = %d, want 0", len(sess.Messages))
	}
	if sess.HasUserActivity {
		t.Error("has_user_activity = true, want false")
	}
}

func TestAddMessage(t *testing.T) {
	sess := NewSession(NewSessionInput{Model: "test"})

	msg := types.NewMessage(types.MsgUser, "hello world")
	sess.AddMessage(msg)

	if len(sess.Messages) != 1 {
		t.Fatalf("messages length = %d, want 1", len(sess.Messages))
	}
	if sess.Messages[0].Type != types.MsgUser {
		t.Errorf("type = %q, want %q", sess.Messages[0].Type, types.MsgUser)
	}
	if sess.Messages[0].Content != "hello world" {
		t.Errorf("content = %q, want %q", sess.Messages[0].Content, "hello world")
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	tmpDir := t.TempDir()

	sess := NewSession(NewSessionInput{
		Model:    "test-model",
		Endpoint: "http://localhost:8000/v1",
		CWD:      "/tmp",
	})

	userMsg := types.NewMessage(types.MsgUser, "test prompt")
	sess.AddMessage(userMsg)

	assistantMsg := types.NewMessage(types.MsgAssistant, "test response")
	assistantMsg.ThinkingText = "thinking..."
	sess.AddMessage(assistantMsg)

	sess.HasUserActivity = true

	path := filepath.Join(tmpDir, "session_test.json")
	if err := sess.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	if loaded.Metadata.Model != "test-model" {
		t.Errorf("model = %q, want %q", loaded.Metadata.Model, "test-model")
	}
	if loaded.Config.MaxContextTokens != sess.Config.MaxContextTokens {
		t.Errorf("max_context_tokens mismatch")
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(loaded.Messages))
	}
	if loaded.Messages[0].Type != types.MsgUser {
		t.Errorf("first message type = %q, want %q", loaded.Messages[0].Type, types.MsgUser)
	}
	if loaded.Messages[1].ThinkingText != "thinking..." {
		t.Errorf("thinking text = %q, want %q", loaded.Messages[1].ThinkingText, "thinking...")
	}
	if !loaded.HasUserActivity {
		t.Error("has_user_activity = false, want true")
	}
}

func TestLoadSessionNotFound(t *testing.T) {
	_, err := LoadSession("/nonexistent/session.json")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestLoadSessionCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "corrupt.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSession(path)
	if err == nil {
		t.Error("expected error for corrupted file, got nil")
	}
}

func TestToAppMessages(t *testing.T) {
	sess := NewSession(NewSessionInput{Model: "test"})

	userMsg := types.NewMessage(types.MsgUser, "hello")
	userMsg.ThinkingText = "some thinking"
	userMsg.Collapsed = true
	sess.AddMessage(userMsg)

	appMsgs := sess.ToAppMessages()
	if len(appMsgs) != 1 {
		t.Fatalf("app messages length = %d, want 1", len(appMsgs))
	}
	if appMsgs[0].Content != "hello" {
		t.Errorf("content = %q, want %q", appMsgs[0].Content, "hello")
	}
	if appMsgs[0].ThinkingText != "some thinking" {
		t.Errorf("thinking = %q, want %q", appMsgs[0].ThinkingText, "some thinking")
	}
	if !appMsgs[0].Collapsed {
		t.Error("collapsed = false, want true")
	}
}

func TestToLLMMessages_UserOnly(t *testing.T) {
	sess := NewSession(NewSessionInput{Model: "test"})

	sess.AddMessage(types.NewMessage(types.MsgUser, "hello"))
	sess.AddMessage(types.NewMessage(types.MsgAssistant, "hi there"))

	llmMsgs := sess.ToLLMMessages()
	if len(llmMsgs) != 2 {
		t.Fatalf("llm messages length = %d, want 2", len(llmMsgs))
	}
	if llmMsgs[0].Role != "user" {
		t.Errorf("first role = %q, want %q", llmMsgs[0].Role, "user")
	}
	if llmMsgs[1].Role != "assistant" {
		t.Errorf("second role = %q, want %q", llmMsgs[1].Role, "assistant")
	}
}

func TestToLLMMessages_SkipsSystemMessages(t *testing.T) {
	sess := NewSession(NewSessionInput{Model: "test"})

	sess.AddMessage(types.NewMessage(types.MsgUser, "hello"))
	sess.AddMessage(types.NewMessage(types.MsgAssistant, "hi"))
	sess.AddMessage(types.NewMessage(types.MsgSystem, "turn stats"))
	sess.AddMessage(types.NewMessage(types.MsgUser, "second prompt"))

	llmMsgs := sess.ToLLMMessages()
	if len(llmMsgs) != 3 {
		t.Fatalf("llm messages length = %d, want 3 (system should be skipped)", len(llmMsgs))
	}
}

func TestToLLMMessages_ToolCalls(t *testing.T) {
	sess := NewSession(NewSessionInput{Model: "test"})

	sess.AddMessage(types.NewMessage(types.MsgUser, "read a file"))

	// Assistant message with tool calls
	sess.AddMessage(types.NewMessage(types.MsgAssistant, ""))
	sess.AddMessage(types.NewToolCall("read", `{"path":"test.txt"}`))
	sess.AddMessage(types.NewToolResult("read", "file content", false))

	sess.AddMessage(types.NewMessage(types.MsgAssistant, "The file contains: file content"))

	llmMsgs := sess.ToLLMMessages()

	// Should have: user, assistant (with tool_calls), tool result, assistant
	if len(llmMsgs) != 4 {
		t.Fatalf("llm messages length = %d, want 4", len(llmMsgs))
	}

	// Check assistant message has tool calls
	if len(llmMsgs[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool calls = %d, want 1", len(llmMsgs[1].ToolCalls))
	}
	if llmMsgs[1].ToolCalls[0].Function.Name != "read" {
		t.Errorf("tool name = %q, want %q", llmMsgs[1].ToolCalls[0].Function.Name, "read")
	}

	// Check tool result
	if llmMsgs[2].Role != "tool" {
		t.Errorf("third role = %q, want %q", llmMsgs[2].Role, "tool")
	}
	if llmMsgs[2].Content != "file content" {
		t.Errorf("tool result content = %q, want %q", llmMsgs[2].Content, "file content")
	}
}

func TestToSessionMessage_RoundTrip(t *testing.T) {
	original := &types.Message{
		ID:               "test-123",
		Type:             types.MsgToolResult,
		Content:          "result content",
		Summary:          "✓ read: 5 lines",
		ThinkingText:     "thinking",
		ToolName:         "read",
		ToolArgs:         `{"path":"test.txt"}`,
		IsError:          true,
		Collapsed:        true,
		ThinkingExpanded: true,
		Timestamp:        time.Date(2024, 1, 15, 14, 30, 22, 0, time.UTC),
		IsTurnStats:      false,
	}

	sm := ToSessionMessage(original)
	restored := ToAppMessage(&sm)

	if restored.ID != original.ID {
		t.Errorf("ID = %q, want %q", restored.ID, original.ID)
	}
	if restored.Type != original.Type {
		t.Errorf("Type = %q, want %q", restored.Type, original.Type)
	}
	if restored.Content != original.Content {
		t.Errorf("Content = %q, want %q", restored.Content, original.Content)
	}
	if restored.Summary != original.Summary {
		t.Errorf("Summary = %q, want %q", restored.Summary, original.Summary)
	}
	if restored.IsError != original.IsError {
		t.Errorf("IsError = %v, want %v", restored.IsError, original.IsError)
	}
	if restored.Collapsed != original.Collapsed {
		t.Errorf("Collapsed = %v, want %v", restored.Collapsed, original.Collapsed)
	}
	if restored.ThinkingExpanded != original.ThinkingExpanded {
		t.Errorf("ThinkingExpanded = %v, want %v", restored.ThinkingExpanded, original.ThinkingExpanded)
	}
}

func TestGetSessionDir(t *testing.T) {
	dir, err := GetSessionDir()
	if err != nil {
		t.Fatalf("GetSessionDir: %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty session directory path")
	}
	// Should end with "sessions"
	if filepath.Base(dir) != "sessions" {
		t.Errorf("session dir base = %q, want %q", filepath.Base(dir), "sessions")
	}
}

func TestEnsureSessionDir(t *testing.T) {
	// Use a temp home dir by temporarily modifying the logic
	// Just verify it doesn't error
	if err := EnsureSessionDir(); err != nil {
		t.Fatalf("EnsureSessionDir: %v", err)
	}
}

func TestGenerateSessionFilename(t *testing.T) {
	filename := GenerateSessionFilename()
	if len(filename) == 0 {
		t.Error("expected non-empty filename")
	}
	if !strings.HasSuffix(filename, ".json") {
		t.Errorf("filename should end with .json, got %q", filename)
	}
	// Should match pattern: session_YYYYMMDD_HHMMSS.json
	expectedPrefix := "session_"
	if len(filename) < len(expectedPrefix) || filename[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("filename should start with %q, got %q", expectedPrefix, filename)
	}
}

func TestListSessions(t *testing.T) {
	// Create a temp session directory
	tmpDir := t.TempDir()
	sessDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some session files
	if err := os.WriteFile(filepath.Join(sessDir, "session_20240115_143022.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessDir, "session_20240115_150045.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Non-json file should be ignored
	if err := os.WriteFile(filepath.Join(sessDir, "notes.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// We can't easily test ListSessions with a custom dir, but we can verify
	// it doesn't panic and returns a slice
	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	// sessions should be from ~/.mok/sessions/ (not our temp dir)
	// but at least we verified it runs without error
	_ = sessions
}

func TestDeleteSession(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.json")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteSession(path); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	err := DeleteSession("/nonexistent/session.json")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestVersionMigration(t *testing.T) {
	// Test that sessions with empty version get assigned current version
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.json")
	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	sess, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	if sess.Metadata.Version != CurrentVersion {
		t.Errorf("version = %q, want %q (should default to current)", sess.Metadata.Version, CurrentVersion)
	}
}
