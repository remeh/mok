package compaction

import (
	"testing"

	"github.com/user/mok/internal/llm"
)

func TestShouldCompact(t *testing.T) {
	tests := []struct {
		name      string
		tokens    int
		maxTokens int
		threshold float64
		want      bool
	}{
		{"under threshold", 80000, 131072, 0.8, false},
		{"at threshold", 104857, 131072, 0.8, false},
		{"over threshold", 104858, 131072, 0.8, true},
		{"no max tokens", 100000, 0, 0.8, false},
		{"zero threshold", 100000, 131072, 0.0, false},
		{"high threshold", 117000, 131072, 0.9, false},
		{"very high usage", 125000, 131072, 0.8, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldCompact(tt.tokens, tt.maxTokens, tt.threshold); got != tt.want {
				t.Errorf("ShouldCompact(%d, %d, %.2f) = %v, want %v",
					tt.tokens, tt.maxTokens, tt.threshold, got, tt.want)
			}
		})
	}
}

func TestFindCutPoint(t *testing.T) {
	// Create test messages
	messages := []llm.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello, can you help me with a project?"},
		{Role: "assistant", Content: "Of course! What do you need help with?"},
		{Role: "user", Content: "I need to write a Go program."},
		{Role: "assistant", Content: "Sure, I can help with that. What should the program do?"},
		{Role: "user", Content: "It should read a file and count the lines."},
		{Role: "assistant", Content: "I can write that for you. Let me create the code."},
	}

	tests := []struct {
		name         string
		targetTokens int
		keepRecent   int
		wantCutIndex int
	}{
		{"no cut needed", 10000, 1000, len(messages)},
		{"cut some - cut to fit target with keepRecent", 50, 20, 5},
		{"cut more - cut more aggressively", 30, 10, 6},
		{"keep recent only - prioritize keepRecent over target", 20, 50, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cutPoint := FindCutPoint(messages, tt.targetTokens, tt.keepRecent)
			if cutPoint.RemoveBeforeIndex != tt.wantCutIndex {
				t.Errorf("FindCutPoint() = %d, want %d", cutPoint.RemoveBeforeIndex, tt.wantCutIndex)
			}
		})
	}
}

func TestApplyCut(t *testing.T) {
	messages := []llm.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "user1"},
		{Role: "assistant", Content: "assistant1"},
		{Role: "user", Content: "user2"},
		{Role: "assistant", Content: "assistant2"},
	}

	// RemoveBeforeIndex: 2 means keep from index 2 onwards
	// Result should be: [assistant1, user2, assistant2]
	cutPoint := CutPoint{RemoveBeforeIndex: 2}
	result := ApplyCut(messages, cutPoint)

	if len(result) != 3 {
		t.Errorf("ApplyCut() length = %d, want 3", len(result))
	}

	if result[0].Role != "assistant" || result[0].Content != "assistant1" {
		t.Errorf("ApplyCut() first message = %v, want assistant1", result[0])
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	messages := []llm.Message{
		{Role: "system", Content: "Hello"},
		{Role: "user", Content: "World"},
		{Role: "assistant", Content: "Test"},
	}

	tokens := EstimateMessagesTokens(messages)
	// Each message: content tokens + 4 overhead
	// "Hello" = 2 tokens + 4 = 6
	// "World" = 2 tokens + 4 = 6
	// "Test" = 1 token + 4 = 5
	// Total = 17
	if tokens < 15 || tokens > 20 {
		t.Errorf("EstimateMessagesTokens() = %d, expected ~17", tokens)
	}
}

func TestExtractFileOps(t *testing.T) {
	messages := []llm.Message{
		{
			Role:    "assistant",
			Content: "I'll read the file for you.",
			ToolCalls: []llm.APIToolCall{
				{
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "read",
						Arguments: `{"path": "main.go"}`,
					},
				},
			},
		},
	}

	ops := ExtractFileOps(messages)
	if len(ops.ReadFiles) != 1 {
		t.Errorf("ExtractFileOps() ReadFiles = %v, want [main.go]", ops.ReadFiles)
	}
}

func TestBuildSummaryText(t *testing.T) {
	fileOps := FileOperations{
		ReadFiles:    []string{"main.go", "config.yaml"},
		WrittenFiles: []string{"output.txt"},
		EditedFiles:  []string{"README.md"},
	}

	summary := BuildSummaryText("Build a Go app", "User wants to create a simple application",
		"Code has been written", "Test the application", fileOps)

	if summary == "" {
		t.Error("BuildSummaryText() returned empty string")
	}

	if !contains(summary, "## Goal") {
		t.Error("BuildSummaryText() missing Goal section")
	}

	if !contains(summary, "main.go") {
		t.Error("BuildSummaryText() missing file list")
	}
}

func TestExtractSummaryContent(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"with tags", "<summary>Hello World</summary>", "Hello World"},
		{"with whitespace", "  <summary>  Trimmed  </summary>  ", "Trimmed"},
		{"without tags", "Plain text", "Plain text"},
		{"empty tags", "<summary></summary>", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSummaryContent(tt.text)
			if got != tt.want {
				t.Errorf("extractSummaryContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
