package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080/v1", "test-token")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("BaseURL = %q, want 'http://localhost:8080/v1'", client.BaseURL)
	}
	if client.BearerToken != "test-token" {
		t.Errorf("BearerToken = %q, want 'test-token'", client.BearerToken)
	}
	if client.httpClient == nil {
		t.Error("httpClient not set")
	}
}

func TestNewClientNoToken(t *testing.T) {
	client := NewClient("http://localhost:8080/v1", "")

	if client.BearerToken != "" {
		t.Errorf("BearerToken = %q, want ''", client.BearerToken)
	}
}

func TestChatRequestMarshal(t *testing.T) {
	req := ChatRequest{
		Model:     "test-model",
		Messages:  []Message{{Role: "user", Content: "hello"}},
		MaxTokens: 100,
	}

	body := req
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var unmarshaled ChatRequest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if unmarshaled.Model != "test-model" {
		t.Errorf("model = %q, want 'test-model'", unmarshaled.Model)
	}
	if unmarshaled.MaxTokens != 100 {
		t.Errorf("max_tokens = %d, want 100", unmarshaled.MaxTokens)
	}
	if len(unmarshaled.Messages) != 1 {
		t.Errorf("messages count = %d, want 1", len(unmarshaled.Messages))
	}
}

func TestStreamText(t *testing.T) {
	chunks := []struct {
		role    string
		content string
	}{
		{"assistant", ""},
		{"", "Hello"},
		{"", " "},
		{"", "world"},
		{"", "!"},
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		resp := map[string]any{
			"choices": []any{
				map[string]any{
					"delta": map[string]any{
						"role":    chunk.role,
						"content": chunk.content,
					},
				},
			},
		}
		data, _ := json.Marshal(resp)
		sb.WriteString(fmt.Sprintf("data: %s\n", string(data)))
	}
	// Usage chunk
	usageChunk := map[string]any{
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}
	usageData, _ := json.Marshal(usageChunk)
	sb.WriteString(fmt.Sprintf("data: %s\n", string(usageData)))
	sb.WriteString("data: [DONE]\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sb.String())
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "")
	req := &ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}

	events, err := client.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	var received []string
	var gotDone bool
	for event := range events {
		switch event.Type {
		case "text":
			received = append(received, event.Text)
		case "done":
			gotDone = true
			if event.Usage == nil {
				t.Error("expected usage in done event")
			} else {
				if event.Usage.TotalTokens != 15 {
					t.Errorf("total_tokens = %d, want 15", event.Usage.TotalTokens)
				}
			}
		}
	}

	if !gotDone {
		t.Error("expected done event")
	}

	fullText := strings.Join(received, "")
	if fullText != "Hello world!" {
		t.Errorf("full text = %q, want 'Hello world!'", fullText)
	}
}

func TestStreamThinking(t *testing.T) {
	var sb strings.Builder

	// Thinking chunk
	thinkChunk := map[string]any{
		"choices": []any{
			map[string]any{
				"delta": map[string]any{
					"reasoning_content": "let me think...",
				},
			},
		},
	}
	data, _ := json.Marshal(thinkChunk)
	sb.WriteString(fmt.Sprintf("data: %s\n", string(data)))

	// Text chunk
	textChunk := map[string]any{
		"choices": []any{
			map[string]any{
				"delta": map[string]any{
					"content": "The answer is 42",
				},
			},
		},
	}
	data, _ = json.Marshal(textChunk)
	sb.WriteString(fmt.Sprintf("data: %s\n", string(data)))

	sb.WriteString("data: [DONE]\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sb.String())
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "")
	req := &ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "what is 6*7?"}},
	}

	events, err := client.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	var thinking, text string
	for event := range events {
		switch event.Type {
		case "thinking":
			thinking += event.ThinkingDelta
		case "text":
			text += event.Text
		}
	}

	if thinking != "let me think..." {
		t.Errorf("thinking = %q, want 'let me think...'", thinking)
	}
	if text != "The answer is 42" {
		t.Errorf("text = %q, want 'The answer is 42'", text)
	}
}

func TestStreamEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n")
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "")
	req := &ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}

	events, err := client.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	// Empty stream should complete without errors
	for event := range events {
		if event.Type == "error" {
			t.Errorf("unexpected error event: %v", event.Err)
		}
	}
}

func TestStreamMalformedChunks(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("data: {invalid json}\n")
	sb.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"valid\"}}]}\n")
	sb.WriteString("data: [DONE]\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sb.String())
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "")
	req := &ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}

	events, err := client.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	var text string
	for event := range events {
		if event.Type == "text" {
			text += event.Text
		}
	}

	if text != "valid" {
		t.Errorf("text = %q, want 'valid'", text)
	}
}

func TestStreamURLConstruction(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n")
	}))
	defer server.Close()

	// Test with trailing slash
	client := NewClient(server.URL+"/", "")
	req := &ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}
	events, _ := client.Stream(context.Background(), req)
	for range events {
	}

	if receivedPath != "/chat/completions" {
		t.Errorf("path = %q, want '/chat/completions'", receivedPath)
	}
}

func TestStreamBearerToken(t *testing.T) {
	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: [DONE]\n")
	}))
	defer server.Close()

	client := NewClient(server.URL, "my-secret-token")
	req := &ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}
	events, _ := client.Stream(context.Background(), req)
	for range events {
	}

	if gotToken != "Bearer my-secret-token" {
		t.Errorf("Authorization = %q, want 'Bearer my-secret-token'", gotToken)
	}
}

func TestStreamAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"internal error"}}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	req := &ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}

	_, err := client.Stream(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, want it to contain '500'", err)
	}
}
