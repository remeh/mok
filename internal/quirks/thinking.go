package quirks

import (
	"github.com/user/mmok/internal/llm"
)

// UseThinkingAsContent returns the content unchanged if it is non-empty.
// When content is empty but thinking text exists, it returns the thinking
// text with possible leaked tag markers stripped. Some models (e.g. qwen3.6-27b-coder)
// put their entire response in reasoning_content with no visible text.
func UseThinkingAsContent(content string, thinking string, debug llm.DebugLogger) (string, bool) {
	if content != "" {
		return content, false
	}
	if thinking == "" {
		return content, false
	}
	debug.Event("QUIRK", "thinking-as-content: using thinking as content")
	return thinking, true
}
