package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseToolArgs parses accumulated raw JSON arguments with repair.
// Returns the parsed object, or nil if all parsing strategies fail.
// On failure, the raw string is still available for error reporting.
func ParseToolArgs(raw string) (json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return json.RawMessage("{}"), nil
	}

	// Layer 1: try direct parse
	if json.Valid([]byte(raw)) {
		return json.RawMessage(raw), nil
	}

	// Layer 2: repair common malformations, then validate
	repaired := RepairJSON(raw)
	if json.Valid([]byte(repaired)) {
		return json.RawMessage(repaired), nil
	}

	// Layer 3: close unclosed strings/braces, then validate
	closed := CloseJSON(raw)
	if json.Valid([]byte(closed)) {
		return json.RawMessage(closed), nil
	}

	return nil, fmt.Errorf("failed to parse tool arguments: %s", raw)
}

// RepairJSON fixes common JSON malformations from LLM output:
// - Escapes raw control characters inside strings
// - Doubles backslashes before invalid escape characters
func RepairJSON(raw string) string {
	result := make([]byte, 0, len(raw))
	inString := false
	escaped := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if escaped {
			result = append(result, ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			if inString {
				// Check if next char is a valid JSON escape
				if i+1 < len(raw) && isValidEscape(raw[i+1]) {
					result = append(result, ch)
					escaped = true
					continue
				}
				// Invalid escape: double the backslash
				result = append(result, '\\', '\\')
				continue
			}
			result = append(result, ch)
			continue
		}

		if ch == '"' {
			inString = !inString
			result = append(result, ch)
			continue
		}

		// Inside a string: escape raw control characters
		if inString && ch < 0x20 {
			switch ch {
			case '\b':
				result = append(result, '\\', 'b')
			case '\f':
				result = append(result, '\\', 'f')
			case '\n':
				result = append(result, '\\', 'n')
			case '\r':
				result = append(result, '\\', 'r')
			case '\t':
				result = append(result, '\\', 't')
			default:
				// Use \u00XX for other control chars
				result = append(result, fmt.Sprintf("\\u%04x", ch)...)
			}
			continue
		}

		result = append(result, ch)
	}

	return string(result)
}

// CloseJSON closes unclosed strings, arrays, and objects:
//
//	{"key": "val  →  {"key": "val"}
//	["a", "b"     →  ["a", "b"]
func CloseJSON(raw string) string {
	// First repair any common malformations
	fixed := RepairJSON(raw)

	// Track open strings, arrays, and objects
	inString := false
	escaped := false
	openBraces := 0
	openBrackets := 0
	openStrings := 0

	for i := 0; i < len(fixed); i++ {
		ch := fixed[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			if inString {
				inString = false
			} else {
				inString = true
			}
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{':
			openBraces++
		case '}':
			openBraces--
		case '[':
			openBrackets++
		case ']':
			openBrackets--
		}
	}

	// If we ended inside a string, close it
	if inString {
		openStrings++
	}

	result := fixed

	// Close unclosed strings
	for i := 0; i < openStrings; i++ {
		result += "\""
	}

	// Close unclosed arrays
	for i := 0; i < openBrackets; i++ {
		result += "]"
	}

	// Close unclosed objects
	for i := 0; i < openBraces; i++ {
		result += "}"
	}

	return result
}

// isValidEscape returns true if the character is a valid JSON escape character.
func isValidEscape(ch byte) bool {
	switch ch {
	case '"', '/', 'b', 'f', 'n', 'r', 't', 'u':
		return true
	case 92: // backslash
		return true
	default:
		return false
	}
}
