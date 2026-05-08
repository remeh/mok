package quirks

import (
	"fmt"
	"strings"

	"github.com/user/mok/internal/llm"
)

// SanitizedToolCalls holds the outcome of sanitizing tool call arguments.
type SanitizedToolCalls struct {
	// Valid are tool calls with repaired, valid JSON args.
	Valid []*llm.PartialTC
	// DroppedNames lists the tool names that had unrepairable args.
	DroppedNames []string
}

// SanitizeToolCalls validates and repairs tool call arguments before they
// are stored in conversation history. This prevents malformed JSON (e.g. a
// truncated "{") from corrupting the history and causing server-side 500
// errors on replay.
//
// For each tool call:
//   - If args parse/repair successfully, the repaired JSON is written back.
//   - If args are unrepairable, the tool call is dropped.
//
// Returns the sanitized result and a retry notice message to inject into
// history when tool calls were dropped (empty string if none were dropped).
func SanitizeToolCalls(toolCalls []*llm.PartialTC, debug llm.DebugLogger) (SanitizedToolCalls, string) {
	var result SanitizedToolCalls

	// Deduplicate: Qwen sometimes emits the same tool call multiple times.
	// We keep the first occurrence of each (Name, RawArgs) pair.
	seen := make(map[string]bool)

	for _, tc := range toolCalls {
		key := tc.Name + "\x00" + tc.RawArgs
		if seen[key] {
			debug.Event("QUIRK", "malformed-tool-call: dedup removed duplicate %s", tc.Name)
			continue
		}
		seen[key] = true

		repaired, err := llm.ParseToolArgs(tc.RawArgs)
		if err != nil {
			result.DroppedNames = append(result.DroppedNames, tc.Name)
			debug.Event("QUIRK", "malformed-tool-call: dropped %s, unrepairable args: %s",
				tc.Name, truncateQuirk(tc.RawArgs, 60))
			continue
		}

		tc.RawArgs = string(repaired)
		result.Valid = append(result.Valid, tc)
	}

	var notice string
	if len(result.DroppedNames) > 0 {
		notice = fmt.Sprintf(
			"Your tool call(s) for %s had malformed arguments and could not be executed. Please retry with valid arguments.",
			strings.Join(result.DroppedNames, ", "),
		)
		debug.Event("QUIRK", "malformed-tool-call: injecting retry notice for: %v", result.DroppedNames)
	}

	return result, notice
}

func truncateQuirk(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
