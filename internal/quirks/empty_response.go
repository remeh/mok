package quirks

import (
	"github.com/user/mmok/internal/llm"
)

// MaxEmptyRetries is the maximum number of times to retry when the model
// returns an empty response (stop reason "stop" but no text, thinking, or
// tool calls). Some models occasionally emit only an EOS token.
const MaxEmptyRetries = 2

// IsEmptyResponse returns true when the model completed normally (stop)
// but produced no usable output — no text, no thinking, and no tool calls.
func IsEmptyResponse(stopReason string, textLen int, thinkingLen int, toolCalls int, debug llm.DebugLogger) bool {
	if stopReason != "stop" {
		return false
	}
	if textLen > 0 || thinkingLen > 0 || toolCalls > 0 {
		return false
	}
	if debug != nil {
		debug.Event("QUIRK", "empty-response: model returned stop with no content")
	}
	return true
}
