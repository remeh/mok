package quirks

import (
	"strings"

	"github.com/user/mok/internal/llm"
)

// reasoningTags lists known reasoning/thinking XML tag markers across model
// families. SanitizeContent strips these markers from text while preserving
// the content between them.
var reasoningTags = []string{
	"think", "thinking", "thought", "Thought",
	"reasoning", "analysis", "reflection",
	"inner_thoughts", "scratchpad", "chain_of_thought",
	"tool_call",
}

// SanitizeContent strips leaked reasoning/thinking XML tag markers from text,
// preserving the content between them. Always applied to all assistant
// content — not model-specific. Logs when changes are made.
func SanitizeContent(s string, debug llm.DebugLogger) (string, bool) {
	original := s

	for _, tag := range reasoningTags {
		s = strings.ReplaceAll(s, "<"+tag+">", "")
		s = strings.ReplaceAll(s, "</"+tag+">", "")
	}

	s = xmlToolCallPattern.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)

	changed := s != original
	if changed {
		debug.Event("QUIRK", "sanitize: removed leaked reasoning tags from content")
	}
	return s, changed
}
