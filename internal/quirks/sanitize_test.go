package quirks

import (
	"testing"

	"github.com/user/mok/internal/llm"
)

// mockDebug implements llm.DebugLogger for testing.
type mockDebug struct {
	events []string
}

func (m *mockDebug) Debug(_ string, _ string, _ ...any)    {}
func (m *mockDebug) Info(_ string, _ string, _ ...any)     {}
func (m *mockDebug) Request(_ string, _ string, _ ...any)  {}
func (m *mockDebug) Response(_ string, _ string, _ ...any) {}
func (m *mockDebug) Tool(_ string, _ string, _ ...any)     {}
func (m *mockDebug) JSON(_ string, _ string, _ any)        {}
func (m *mockDebug) Dump(_ string, _ string, _ []byte)     {}
func (m *mockDebug) Event(category string, format string, args ...any) {
	m.events = append(m.events, category+":"+format)
}

func TestSanitizeContent(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"no tags":            {"Hello, this is normal content.", "Hello, this is normal content."},
		"thinking":           {"<thinking>let me think...</thinking>Hello world", "let me think...Hello world"},
		"reasoning":          {"<reasoning>some reasoning</reasoning>Answer", "some reasoningAnswer"},
		"deepseek think":     {"<think>reasoning step by step...</think>The answer is 42.", "reasoning step by step...The answer is 42."},
		"reflection":         {"<reflection>let me reconsider</reflection>Actually, the answer is 7.", "let me reconsiderActually, the answer is 7."},
		"scratchpad":         {"<scratchpad>working notes...</scratchpad>Final result.", "working notes...Final result."},
		"inner_thoughts":     {"<inner_thoughts>hmm...</inner_thoughts>Here is my answer.", "hmm...Here is my answer."},
		"chain_of_thought":   {"<chain_of_thought>step 1, step 2</chain_of_thought>Done.", "step 1, step 2Done."},
		"unclosed think":     {"Hello <think>this was never closed", "Hello this was never closed"},
		"multiple tags":      {"<thinking>think1</thinking>text<thought>think2</thought>more", "think1textthink2more"},
		"mixed tag families": {"<think>deep reasoning</think>answer<reflection>wait no</reflection>final", "deep reasoninganswerwait nofinal"},
		"trims whitespace":   {"<thinking>think</thinking>  content  ", "think  content"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, _ := SanitizeContent(tc.input, llm.NopLogger{})
			if result != tc.want {
				t.Errorf("expected %q, got %q", tc.want, result)
			}
		})
	}
}

func TestSanitizeContent_DebugLog(t *testing.T) {
	debug := &mockDebug{}
	SanitizeContent("</think>content", debug)
	if len(debug.events) == 0 {
		t.Error("expected debug event when tags are removed")
	}
}

func TestSanitizeContent_NoDebugLogWhenClean(t *testing.T) {
	debug := &mockDebug{}
	SanitizeContent("clean content", debug)
	if len(debug.events) > 0 {
		t.Error("expected no debug event when no tags removed")
	}
}

func TestUseThinkingAsContent(t *testing.T) {
	tests := map[string]struct {
		content  string
		thinking string
		want     string
	}{
		"content present": {"real content", "thinking text", "real content"},
		"empty content":   {"", "<thinking>fallback</thinking>", "<thinking>fallback</thinking>"},
		"both empty":      {"", "", ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, _ := UseThinkingAsContent(tc.content, tc.thinking, llm.NopLogger{})
			if result != tc.want {
				t.Errorf("expected %q, got %q", tc.want, result)
			}
		})
	}
}

func TestUseThinkingAsContent_DebugLog(t *testing.T) {
	debug := &mockDebug{}
	UseThinkingAsContent("", "thinking text", debug)
	if len(debug.events) == 0 {
		t.Error("expected debug event when thinking is used as content")
	}
}
