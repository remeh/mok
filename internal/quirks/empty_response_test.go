package quirks

import (
	"testing"

	"github.com/user/mmok/internal/llm"
)

func TestIsEmptyResponse(t *testing.T) {
	tests := []struct {
		name        string
		stopReason  string
		textLen     int
		thinkingLen int
		toolCalls   int
		want        bool
	}{
		{"empty stop", "stop", 0, 0, 0, true},
		{"has text", "stop", 10, 0, 0, false},
		{"has thinking", "stop", 0, 5, 0, false},
		{"has tool calls", "stop", 0, 0, 1, false},
		{"tool_calls stop reason", "tool_calls", 0, 0, 0, false},
		{"length stop reason", "length", 0, 0, 0, false},
		{"empty string stop reason", "", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEmptyResponse(tt.stopReason, tt.textLen, tt.thinkingLen, tt.toolCalls, llm.NopLogger{})
			if got != tt.want {
				t.Errorf("IsEmptyResponse(%q, %d, %d, %d) = %v, want %v",
					tt.stopReason, tt.textLen, tt.thinkingLen, tt.toolCalls, got, tt.want)
			}
		})
	}
}
