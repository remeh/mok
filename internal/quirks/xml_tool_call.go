package quirks

import (
	"regexp"
	"strings"

	"github.com/user/mmok/internal/llm"
)

// xmlToolCallPattern matches XML-style tool calls emitted by some models
// (e.g. Qwen) instead of proper JSON tool_calls.
//
// The pattern matches:
//   <function=name>
//   <parameter=key>value</parameter>
//   </function>
var xmlToolCallPattern = regexp.MustCompile(`(?s)<function=([^<]+)>(.*?)</function>`)

// paramPattern matches individual parameters within an XML tool call.
var paramPattern = regexp.MustCompile(`<parameter=([^>]+)>(.*?)</parameter>`)

// QwenXMLToolCall represents a parsed XML-style tool call.
type QwenXMLToolCall struct {
	Name string
	Args map[string]string
}

// ExtractXMLToolCalls scans text for XML-style tool calls.
// Returns parsed tool calls and whether any were found.
// Only call this when the standard JSON tool_call mechanism produced nothing.
func ExtractXMLToolCalls(text string, debug llm.DebugLogger) ([]QwenXMLToolCall, bool) {
	if text == "" {
		return nil, false
	}

	matches := xmlToolCallPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, false
	}

	var result []QwenXMLToolCall
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		name := strings.TrimSpace(match[1])
		paramsText := match[2]

		args := make(map[string]string)
		paramMatches := paramPattern.FindAllStringSubmatch(paramsText, -1)
		for _, pm := range paramMatches {
			if len(pm) >= 3 {
				args[pm[1]] = strings.TrimSpace(pm[2])
			}
		}

		result = append(result, QwenXMLToolCall{
			Name: name,
			Args: args,
		})
	}

	if len(result) > 0 {
		debug.Event("QUIRK", "xml-tool-call: extracted %d XML tool call(s) from text", len(result))
	}

	return result, len(result) > 0
}

// XMLToolCallArgsToJSON converts XML tool call args map to a JSON string.
func XMLToolCallArgsToJSON(args map[string]string) string {
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for k, v := range args {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(`"`)
		sb.WriteString(k)
		sb.WriteString(`":`)
		sb.WriteString(`"`)
		sb.WriteString(strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`))
		sb.WriteString(`"`)
	}
	sb.WriteString("}")
	return sb.String()
}
