package quirks

import (
	"strings"
	"testing"

	"github.com/user/mmok/internal/llm"
)

func TestExtractXMLToolCalls_Empty(t *testing.T) {
	result, found := ExtractXMLToolCalls("", llm.NopLogger{})
	if found {
		t.Error("expected no matches for empty string")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestExtractXMLToolCalls_NoToolCalls(t *testing.T) {
	result, found := ExtractXMLToolCalls("just some regular text", llm.NopLogger{})
	if found {
		t.Error("expected no matches for regular text")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestExtractXMLToolCalls_BashToolCall(t *testing.T) {
	input := "\u2573" +
		"<function=bash>" +
		"<parameter=command>cd /Users/remy/docs/code/mmok && go build -o mmok cmd/mmok/main.go 2>&1; echo \"EXIT: $?\"</parameter>" +
		"</function>" +
		"\u2581"

	result, found := ExtractXMLToolCalls(input, llm.NopLogger{})
	if !found {
		t.Fatal("expected to find XML tool call")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Name != "bash" {
		t.Errorf("expected name 'bash', got %q", result[0].Name)
	}
	if result[0].Args["command"] != "cd /Users/remy/docs/code/mmok && go build -o mmok cmd/mmok/main.go 2>&1; echo \"EXIT: $?\"" {
		t.Errorf("unexpected command: %q", result[0].Args["command"])
	}
}

func TestExtractXMLToolCalls_ReadToolCall(t *testing.T) {
	input := "\u2573" +
		"<function=read>" +
		"<parameter=limit>50</parameter>" +
		"<parameter=offset>1</parameter>" +
		"<parameter=path>/Users/remy/docs/code/mmok/internal/app/app.go</parameter>" +
		"</function>" +
		"\u2581"

	result, found := ExtractXMLToolCalls(input, llm.NopLogger{})
	if !found {
		t.Fatal("expected to find XML tool call")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Name != "read" {
		t.Errorf("expected name 'read', got %q", result[0].Name)
	}
	if result[0].Args["path"] != "/Users/remy/docs/code/mmok/internal/app/app.go" {
		t.Errorf("unexpected path: %q", result[0].Args["path"])
	}
	if result[0].Args["offset"] != "1" {
		t.Errorf("unexpected offset: %q", result[0].Args["offset"])
	}
	if result[0].Args["limit"] != "50" {
		t.Errorf("unexpected limit: %q", result[0].Args["limit"])
	}
}

func TestExtractXMLToolCalls_MultipleToolCalls(t *testing.T) {
	input := "some text " +
		"\u2573" +
		"<function=bash>" +
		"<parameter=command>ls -la</parameter>" +
		"</function>" +
		"\u2581" +
		" more text " +
		"\u2573" +
		"<function=read>" +
		"<parameter=path>file.go</parameter>" +
		"</function>" +
		"\u2581" +
		" end"

	result, found := ExtractXMLToolCalls(input, llm.NopLogger{})
	if !found {
		t.Fatal("expected to find XML tool calls")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].Name != "bash" {
		t.Errorf("expected first name 'bash', got %q", result[0].Name)
	}
	if result[1].Name != "read" {
		t.Errorf("expected second name 'read', got %q", result[1].Name)
	}
}

func TestExtractXMLToolCalls_WithSurroundingText(t *testing.T) {
	input := "I'll run a command to check the files.\n\n" +
		"\u2573" +
		"<function=bash>" +
		"<parameter=command>ls</parameter>" +
		"</function>" +
		"\u2581" +
		"\n\nThat should show the directory contents."

	result, found := ExtractXMLToolCalls(input, llm.NopLogger{})
	if !found {
		t.Fatal("expected to find XML tool call")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Name != "bash" {
		t.Errorf("expected name 'bash', got %q", result[0].Name)
	}
}

func TestExtractXMLToolCalls_MultilineParams(t *testing.T) {
	input := "<tool_call>\n" +
		"<function=read>\n" +
		"<parameter=limit>\n" +
		"50\n" +
		"</parameter>\n" +
		"<parameter=offset>\n" +
		"1\n" +
		"</parameter>\n" +
		"<parameter=path>\n" +
		"/Users/remy/docs/code/mmok/internal/app/app.go\n" +
		"</parameter>\n" +
		"</function>\n" +
		"</tool_call>"

	result, found := ExtractXMLToolCalls(input, llm.NopLogger{})
	if !found {
		t.Fatal("expected to find XML tool call")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Name != "read" {
		t.Errorf("expected name 'read', got %q", result[0].Name)
	}
	if result[0].Args["path"] != "/Users/remy/docs/code/mmok/internal/app/app.go" {
		t.Errorf("unexpected path: %q", result[0].Args["path"])
	}
	if result[0].Args["offset"] != "1" {
		t.Errorf("unexpected offset: %q", result[0].Args["offset"])
	}
	if result[0].Args["limit"] != "50" {
		t.Errorf("unexpected limit: %q", result[0].Args["limit"])
	}
}

func TestXMLToolCallArgsToJSON(t *testing.T) {
	args := map[string]string{
		"path":   "/some/file.go",
		"offset": "1",
	}

	json := XMLToolCallArgsToJSON(args)

	// Check that all keys are present
	if !strings.Contains(json, `"path"`) {
		t.Error("JSON should contain path key")
	}
	if !strings.Contains(json, `"offset"`) {
		t.Error("JSON should contain offset key")
	}
	if !strings.Contains(json, `/some/file.go`) {
		t.Error("JSON should contain path value")
	}
	if !strings.Contains(json, `1`) {
		t.Error("JSON should contain offset value")
	}
}

func TestXMLToolCallArgsToJSON_Escaping(t *testing.T) {
	args := map[string]string{
		"command": `echo "hello" && ls`,
	}

	json := XMLToolCallArgsToJSON(args)

	if !strings.Contains(json, `\"hello\"`) {
		t.Errorf("JSON should have escaped quotes, got: %s", json)
	}
}

func TestXMLToolCallArgsToJSON_Empty(t *testing.T) {
	args := map[string]string{}
	json := XMLToolCallArgsToJSON(args)
	if json != "{}" {
		t.Errorf("expected '{}', got %q", json)
	}
}
