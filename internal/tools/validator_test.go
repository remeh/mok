package tools

import (
	"encoding/json"
	"testing"
)

func TestValidateAndCoerce_StringToNumber(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"count": map[string]any{"type": "number"},
		},
	}
	args := json.RawMessage(`{"count": "42"}`)
	result := ValidateAndCoerce(schema, args)

	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["count"] != float64(42) {
		t.Errorf("count = %v (%T), want 42 (float64)", obj["count"], obj["count"])
	}
}

func TestValidateAndCoerce_StringToInteger(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"line": map[string]any{"type": "integer"},
		},
	}
	args := json.RawMessage(`{"line": "100"}`)
	result := ValidateAndCoerce(schema, args)

	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["line"] != float64(100) {
		t.Errorf("line = %v (%T), want 100 (float64)", obj["line"], obj["line"])
	}
}

func TestValidateAndCoerce_StringToBool(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"force": map[string]any{"type": "boolean"},
		},
	}
	args := json.RawMessage(`{"force": "true"}`)
	result := ValidateAndCoerce(schema, args)

	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["force"] != true {
		t.Errorf("force = %v (%T), want true (bool)", obj["force"], obj["force"])
	}
}

func TestValidateAndCoerce_AlreadyCorrect(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"count": map[string]any{"type": "number"},
		},
	}
	args := json.RawMessage(`{"name": "test", "count": 5}`)
	result := ValidateAndCoerce(schema, args)

	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["name"] != "test" {
		t.Errorf("name = %v, want 'test'", obj["name"])
	}
	if obj["count"] != float64(5) {
		t.Errorf("count = %v, want 5", obj["count"])
	}
}

func TestValidateAndCoerce_EmptyArgs(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	args := json.RawMessage(`{}`)
	result := ValidateAndCoerce(schema, args)
	if string(result) != "{}" {
		t.Errorf("result = %s, want '{}'", result)
	}
}

func TestValidateAndCoerce_InvalidJSON(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	args := json.RawMessage(`not json`)
	result := ValidateAndCoerce(schema, args)
	if string(result) != "not json" {
		t.Errorf("result = %s, want 'not json'", result)
	}
}

func TestValidateAndCoerce_MissingOptional(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"opt":  map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	}
	args := json.RawMessage(`{"name": "test"}`)
	result := ValidateAndCoerce(schema, args)

	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, hasOpt := obj["opt"]; hasOpt {
		t.Error("optional field should not be injected")
	}
}

func TestCoerceValue_NumberToString(t *testing.T) {
	schema := map[string]any{"type": "string"}
	result := coerceValue(float64(42), schema)
	if result != "42" {
		t.Errorf("result = %v, want '42'", result)
	}
}

func TestCoerceValue_BoolToString(t *testing.T) {
	schema := map[string]any{"type": "string"}
	result := coerceValue(true, schema)
	if result != "true" {
		t.Errorf("result = %v, want 'true'", result)
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"yes", true},
		{"no", false},
		{"TRUE", true},
		{"False", false},
		{"maybe", "maybe"},
	}
	for _, tt := range tests {
		result := parseBool(tt.input)
		if result != tt.expected {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		input   string
		typeStr string
		want    any
	}{
		{"42", "integer", 42},
		{"3.14", "number", float64(3.14)},
		{"3.14", "integer", 3},
		{"abc", "number", "abc"},
	}
	for _, tt := range tests {
		result := parseNumber(tt.input, tt.typeStr)
		if result != tt.want {
			t.Errorf("parseNumber(%q, %q) = %v, want %v", tt.input, tt.typeStr, result, tt.want)
		}
	}
}
