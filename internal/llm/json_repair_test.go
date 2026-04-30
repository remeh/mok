package llm

import (
	"encoding/json"
	"testing"
)

func TestParseToolArgs_Valid(t *testing.T) {
	raw := `{"file": "test.go", "line": 42}`
	result, err := ParseToolArgs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["file"] != "test.go" {
		t.Errorf("file = %v, want 'test.go'", obj["file"])
	}
}

func TestParseToolArgs_Empty(t *testing.T) {
	result, err := ParseToolArgs("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != "{}" {
		t.Errorf("result = %s, want '{}'", result)
	}
}

func TestParseToolArgs_Whitespace(t *testing.T) {
	result, err := ParseToolArgs("   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != "{}" {
		t.Errorf("result = %s, want '{}'", result)
	}
}

func TestParseToolArgs_ControlChars(t *testing.T) {
	// Raw newlines inside a string value
	raw := `{"key": "line1\nline2"}`
	result, err := ParseToolArgs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestParseToolArgs_InvalidEscape(t *testing.T) {
	// Invalid escape like \x should be doubled
	raw := `{"key": "val\xue"}`
	result, err := ParseToolArgs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestParseToolArgs_UnclosedBrace(t *testing.T) {
	raw := `{"key": "value"`
	result, err := ParseToolArgs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["key"] != "value" {
		t.Errorf("key = %v, want 'value'", obj["key"])
	}
}

func TestParseToolArgs_UnclosedArray(t *testing.T) {
	raw := `["a", "b"`
	result, err := ParseToolArgs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var arr []any
	if err := json.Unmarshal(result, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(arr) != 2 {
		t.Errorf("len = %d, want 2", len(arr))
	}
}

func TestParseToolArgs_UnclosedString(t *testing.T) {
	raw := `{"key": "unterminated`
	result, err := ParseToolArgs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestParseToolArgs_AllLayersFail(t *testing.T) {
	raw := `not json at all {{{{{{{`
	_, err := ParseToolArgs(raw)
	if err == nil {
		t.Fatal("expected error for completely invalid JSON")
	}
}

func TestRepairJSON_ControlChars(t *testing.T) {
	// Input is inside a JSON string context (between quotes)
	input := `"hello` + string(byte(0)) + `world"`
	result := RepairJSON(input)
	// Should have escaped the null byte
	if !json.Valid([]byte(result)) {
		t.Errorf("result not valid JSON: %q", result)
	}
}

func TestRepairJSON_InvalidEscape(t *testing.T) {
	// Inside a string: \x is invalid, should become \\x
	input := `"hello\xworld"`
	result := RepairJSON(input)
	// \x should become \\x
	expected := `"hello\\xworld"`
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestCloseJSON_UnclosedObject(t *testing.T) {
	input := `{"key": "value"`
	result := CloseJSON(input)
	if !json.Valid([]byte(result)) {
		t.Errorf("result not valid JSON: %s", result)
	}
}

func TestCloseJSON_UnclosedNested(t *testing.T) {
	input := `{"outer": {"inner": "val"`
	result := CloseJSON(input)
	if !json.Valid([]byte(result)) {
		t.Errorf("result not valid JSON: %s", result)
	}
}

func TestCloseJSON_AlreadyValid(t *testing.T) {
	input := `{"key": "value"}`
	result := CloseJSON(input)
	if result != input {
		t.Errorf("result = %s, want %s", result, input)
	}
}

func TestIsValidEscape(t *testing.T) {
	validEscapes := []byte{'"', '/', 'b', 'f', 'n', 'r', 't', 'u', 92} // 92 = backslash
	for _, ch := range validEscapes {
		if !isValidEscape(ch) {
			t.Errorf("isValidEscape(%d) = false, want true", ch)
		}
	}
	invalidEscapes := []byte{'x', 'a', 'z', 0, 127}
	for _, ch := range invalidEscapes {
		if isValidEscape(ch) {
			t.Errorf("isValidEscape(%d) = true, want false", ch)
		}
	}
}
