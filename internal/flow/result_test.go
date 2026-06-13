package flow

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/user/mok/internal/llm"
)

func TestAgentRunResult_Basic(t *testing.T) {
	now := time.Now()
	result := AgentRunResult{
		AgentName:    "coder",
		FinalMessage: "Here is the code.",
		Messages: []llm.Message{
			{Role: "assistant", Content: "Here is the code."},
		},
		TokenUsage: &llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		StartTime: now,
		EndTime:   now.Add(2 * time.Second),
	}

	if result.AgentName != "coder" {
		t.Errorf("AgentName = %q", result.AgentName)
	}
	if !result.Succeeded() {
		t.Error("Succeeded() = false, want true")
	}
	if !result.HasContent() {
		t.Error("HasContent() = false, want true")
	}
	if result.Duration() != 2*time.Second {
		t.Errorf("Duration() = %v, want 2s", result.Duration())
	}
	if result.TokenUsage.TotalTokens != 150 {
		t.Errorf("TokenUsage.TotalTokens = %d, want 150", result.TokenUsage.TotalTokens)
	}
}

func TestAgentRunResult_Error(t *testing.T) {
	result := AgentRunResult{
		AgentName: "senior",
		Error:     errors.New("something went wrong"),
	}

	if result.Succeeded() {
		t.Error("Succeeded() = true, want false")
	}
	if result.AgentName != "senior" {
		t.Errorf("AgentName = %q", result.AgentName)
	}
}

func TestAgentRunResult_EmptyContent(t *testing.T) {
	result := AgentRunResult{
		AgentName:    "reviewer",
		FinalMessage: "",
	}

	if result.HasContent() {
		t.Error("HasContent() = true, want false")
	}
}

func TestAgentRunResult_NoUsage(t *testing.T) {
	result := AgentRunResult{
		AgentName: "tester",
	}

	if result.TokenUsage != nil {
		t.Error("TokenUsage should be nil")
	}
}

func TestBuildHandoffMessage_Full(t *testing.T) {
	opts := HandoffOptions{
		OriginalGoal:      "Write a REST API for the blog.",
		PreviousAgentName: "senior",
		PreviousAgentRole: "software architect",
		Summary: `## Goal
Design the API.

## Files Modified
- api.go (designed)

## Current State
Architecture ready.`,
		CurrentAgentName: "coder",
		CurrentAgentRole: "expert developer",
	}

	msg := BuildHandoffMessage(opts)

	// Verify all sections present
	checks := []string{
		"[Handoff from senior (software architect)]",
		"Here's what was done:",
		"## Goal",
		"api.go (designed)",
		"Architecture ready.",
		"User's original request:",
		"Write a REST API for the blog.",
		"taking over as coder",
		"expert developer",
	}

	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Errorf("handoff message missing %q\nGot:\n%s", check, msg)
		}
	}
}

func TestBuildHandoffMessage_Minimal(t *testing.T) {
	opts := HandoffOptions{
		PreviousAgentName: "agent1",
		PreviousAgentRole: "role1",
		CurrentAgentName:  "agent2",
		CurrentAgentRole:  "role2",
	}

	msg := BuildHandoffMessage(opts)

	if !strings.Contains(msg, "[Handoff from agent1 (role1)]") {
		t.Error("missing handoff header")
	}
	if !strings.Contains(msg, "taking over as agent2") {
		t.Error("missing take-over sentence")
	}

	// Without summary or goal, those sections should be absent
	if strings.Contains(msg, "Here's what was done:") {
		t.Error("should not contain summary section when summary is empty")
	}
	if strings.Contains(msg, "User's original request:") {
		t.Error("should not contain goal section when goal is empty")
	}
}

func TestBuildHandoffMessage_EmptySummary(t *testing.T) {
	opts := HandoffOptions{
		OriginalGoal:      "Fix the bug.",
		PreviousAgentName: "reviewer",
		PreviousAgentRole: "code reviewer",
		Summary:           "",
		CurrentAgentName:  "coder",
		CurrentAgentRole:  "expert developer",
	}

	msg := BuildHandoffMessage(opts)

	// Should have goal but no summary section
	if !strings.Contains(msg, "Fix the bug.") {
		t.Error("missing original goal")
	}
	if strings.Contains(msg, "Here's what was done:") {
		t.Error("should not have summary section")
	}
}

func TestBuildHandoffSummary_WithMessages(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "Add a login endpoint."},
		{Role: "assistant", Content: "I'll add the login endpoint to the API.", ToolCalls: []llm.APIToolCall{
			{ID: "call-1", Type: "function", Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "write", Arguments: `{"path": "auth.go", "content": "package auth..."}`}},
		}},
		{Role: "tool", Content: "File auth.go written successfully.", Name: "write"},
		{Role: "assistant", Content: "Done. The login endpoint is at /api/login."},
	}

	summary := BuildHandoffSummary(messages)

	// Should contain extracted information
	if !strings.Contains(summary, "## Goal") {
		t.Error("summary missing Goal section")
	}
	if !strings.Contains(summary, "Add a login endpoint") {
		t.Errorf("summary missing goal text: %s", summary)
	}
	if !strings.Contains(summary, "## Files Modified") {
		t.Error("summary missing Files Modified section")
	}
	if !strings.Contains(summary, "auth.go") {
		t.Errorf("summary missing file path: %s", summary)
	}
	if !strings.Contains(summary, "## Current State") {
		t.Error("summary missing Current State section")
	}
}

func TestBuildHandoffSummary_Empty(t *testing.T) {
	summary := BuildHandoffSummary(nil)
	if summary != "" {
		t.Errorf("expected empty summary for nil messages, got: %s", summary)
	}

	summary2 := BuildHandoffSummary([]llm.Message{})
	if summary2 != "" {
		t.Errorf("expected empty summary for empty messages, got: %s", summary2)
	}
}

func TestBuildHandoffSummary_ToolCalls(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "Read the config and add logging."},
		{Role: "assistant", Content: "Let me read the config first.", ToolCalls: []llm.APIToolCall{
			{ID: "call-1", Type: "function", Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "read", Arguments: `{"path": "config.yaml"}`}},
		}},
		{Role: "assistant", Content: "Now editing.", ToolCalls: []llm.APIToolCall{
			{ID: "call-2", Type: "function", Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "edit", Arguments: `{"path": "main.go", "oldText": "// old", "newText": "// new"}`}},
		}},
	}

	summary := BuildHandoffSummary(messages)

	if !strings.Contains(summary, "config.yaml") {
		t.Errorf("summary missing read file: %s", summary)
	}
	if !strings.Contains(summary, "main.go") {
		t.Errorf("summary missing edited file: %s", summary)
	}
	if !strings.Contains(summary, "Read the config and add logging") {
		t.Errorf("summary missing goal: %s", summary)
	}
}
