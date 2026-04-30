package tools

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ValidateAndCoerce validates args against schema with type coercion.
// Returns corrected args on success, or the original args on failure
// (never blocks the turn).
// Validation failures are logged as warnings — the tool executor will
// return an error result that the model can retry.
func ValidateAndCoerce(schema map[string]any, args json.RawMessage) json.RawMessage {
	if len(args) == 0 || string(args) == "{}" {
		return args
	}

	var obj map[string]any
	if err := json.Unmarshal(args, &obj); err != nil {
		return args // Can't parse, return as-is
	}

	// Extract properties schema
	properties, _ := schema["properties"].(map[string]any)
	required := make(map[string]bool)
	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}

	for name, propSchema := range properties {
		ps, ok := propSchema.(map[string]any)
		if !ok {
			continue
		}

		if val, exists := obj[name]; exists {
			obj[name] = coerceValue(val, ps)
		}
		// Missing optional fields: omit (don't inject defaults)
	}

	result, err := json.Marshal(obj)
	if err != nil {
		return args
	}
	return json.RawMessage(result)
}

// coerceValue applies type coercion to match what the model likely intended.
func coerceValue(val any, schema map[string]any) any {
	typeStr, _ := schema["type"].(string)
	switch typeStr {
	case "string":
		if s, ok := val.(string); ok {
			return s
		}
		// Coerce number/bool to string
		return toString(val)

	case "number", "integer":
		if f, ok := val.(float64); ok {
			if typeStr == "integer" {
				return int(f)
			}
			return f
		}
		// Coerce string "42" to number 42
		if s, ok := val.(string); ok {
			return parseNumber(s, typeStr)
		}

	case "boolean":
		if b, ok := val.(bool); ok {
			return b
		}
		// Coerce string "true"/"false" to boolean
		if s, ok := val.(string); ok {
			return parseBool(s)
		}

	case "array":
		if _, ok := val.([]any); ok {
			return val
		}

	case "object":
		if _, ok := val.(map[string]any); ok {
			return val
		}
	}

	return val
}

func toString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		if v == float64(int(v)) {
			return strconv.Itoa(int(v))
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		b, _ := json.Marshal(v)
		return strings.TrimSpace(string(b))
	}
}

func parseNumber(s string, typeStr string) any {
	s = strings.TrimSpace(s)
	if typeStr == "integer" {
		if i, err := strconv.Atoi(s); err == nil {
			return i
		}
		// Try float then truncate
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int(f)
		}
	} else {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	// Coercion failed, return original string
	return s
}

func parseBool(s string) any {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return s // Return original string if not recognizable
	}
}
