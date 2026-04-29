package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/mmok/internal/app"
)

func TestNew(t *testing.T) {
	cfg := app.DefaultConfig()
	client := New(cfg)

	if client == nil {
		t.Fatal("New returned nil")
	}
	if client.config != cfg {
		t.Error("config not set")
	}
	if client.client == nil {
		t.Error("http client not set")
	}
}

func TestChatRequestMarshal(t *testing.T) {
	req := ChatRequest{
		Model:       "test-model",
		Messages:    []ChatMsg{{Role: "user", Content: "hello"}},
		Stream:      true,
		Temperature: 0.5,
		MaxTokens:   100,
	}

	data, err := json.Marshal(req)
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
	if !unmarshaled.Stream {
		t.Error("stream should be true")
	}
	if unmarshaled.Temperature != 0.5 {
		t.Errorf("temperature = %f, want 0.5", unmarshaled.Temperature)
	}
	if unmarshaled.MaxTokens != 100 {
		t.Errorf("max_tokens = %d, want 100", unmarshaled.MaxTokens)
	}
	if len(unmarshaled.Messages) != 1 {
		t.Errorf("messages count = %d, want 1", len(unmarshaled.Messages))
	}
}

func TestParseStream(t *testing.T) {
	// Simulate a real SSE stream
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
		resp := ChatResponse{
			Choices: []Choice{
				{
					Delta: Delta{
						Role:    chunk.role,
						Content: chunk.content,
					},
				},
			},
		}
		data, _ := json.Marshal(resp)
		sb.WriteString(fmt.Sprintf("data: %s\n", string(data)))
	}
	sb.WriteString("data: [DONE]\n")

	client := New(app.DefaultConfig())
	var received []string
	fullText, err := client.parseStream(strings.NewReader(sb.String()), func(content string) error {
		received = append(received, content)
		return nil
	})

	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}

	expected := "Hello world!"
	if fullText != expected {
		t.Errorf("fullText = %q, want %q", fullText, expected)
	}

	// Should have received 4 content chunks (skipping the role-only one)
	if len(received) != 4 {
		t.Errorf("received %d chunks, want 4", len(received))
	}
}

func TestParseStreamEarlyError(t *testing.T) {
	chunks := []struct {
		role    string
		content string
	}{
		{"", "Hello"},
		{"", " world"},
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		resp := ChatResponse{
			Choices: []Choice{
				{Delta: Delta{Content: chunk.content}},
			},
		}
		data, _ := json.Marshal(resp)
		sb.WriteString(fmt.Sprintf("data: %s\n", string(data)))
	}
	sb.WriteString("data: [DONE]\n")

	client := New(app.DefaultConfig())
	callCount := 0
	handler := func(content string) error {
		callCount++
		if callCount == 2 {
			return fmt.Errorf("simulated error")
		}
		return nil
	}

	fullText, err := client.parseStream(strings.NewReader(sb.String()), handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if fullText != "Hello" {
		t.Errorf("fullText = %q, want 'Hello' (partial before error)", fullText)
	}
}

func TestParseStreamEmpty(t *testing.T) {
	client := New(app.DefaultConfig())
	fullText, err := client.parseStream(strings.NewReader(""), nil)
	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	if fullText != "" {
		t.Errorf("fullText = %q, want empty", fullText)
	}
}

func TestParseStreamMalformedChunks(t *testing.T) {
	// Mix of valid and invalid chunks
	input := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: not-json
data: {"choices":[{"delta":{"content":" world"}}]}
data: [DONE]
`
	client := New(app.DefaultConfig())
	fullText, err := client.parseStream(strings.NewReader(input), nil)
	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	if fullText != "Hello world" {
		t.Errorf("fullText = %q, want 'Hello world'", fullText)
	}
}

func TestParseStreamNonDataLines(t *testing.T) {
	// SSE can have event:, id:, retry:, etc.
	input := `event: message
id: 1
data: {"choices":[{"delta":{"content":"Hello"}}]}

data: [DONE]
`
	client := New(app.DefaultConfig())
	fullText, err := client.parseStream(strings.NewReader(input), nil)
	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	if fullText != "Hello" {
		t.Errorf("fullText = %q, want 'Hello'", fullText)
	}
}

func TestChatURLConstruction(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "with trailing slash",
			endpoint: "http://localhost:8080/v1/",
			want:     "http://localhost:8080/v1/chat/completions",
		},
		{
			name:     "without trailing slash",
			endpoint: "http://localhost:8080/v1",
			want:     "http://localhost:8080/v1/chat/completions",
		},
		{
			name:     "already has path",
			endpoint: "http://localhost:8080/v1/chat/completions",
			want:     "http://localhost:8080/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &app.Config{
				Endpoint: tt.endpoint,
				Model:    "test",
			}
			client := New(cfg)

			// Use a test server to capture the URL
			var capturedURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedURL = r.URL.String()
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("data: [DONE]\n"))
			}))
			defer server.Close()

			// Override endpoint to point to test server
			client.config.Endpoint = server.URL

			_, _ = client.Chat([]ChatMsg{{Role: "user", Content: "test"}}, nil)

			if capturedURL != "/chat/completions" {
				t.Errorf("URL = %q, want '/chat/completions'", capturedURL)
			}
		})
	}
}

func TestChatWithHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		body, _ := io.ReadAll(r.Body)
		var req ChatRequest
		json.Unmarshal(body, &req)

		if req.Model != "test-model" {
			t.Errorf("model = %q, want 'test-model'", req.Model)
		}
		if !req.Stream {
			t.Error("stream should be true")
		}
		if len(req.Messages) != 1 {
			t.Errorf("messages count = %d, want 1", len(req.Messages))
		}
		if req.Messages[0].Role != "user" {
			t.Errorf("role = %q, want 'user'", req.Messages[0].Role)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for _, word := range []string{"Hello", " ", "world"} {
			resp := ChatResponse{
				Choices: []Choice{{Delta: Delta{Content: word}}},
			}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
		}
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer server.Close()

	cfg := &app.Config{
		Model:    "test-model",
		Endpoint: server.URL,
	}
	client := New(cfg)

	var received []string
	fullText, err := client.Chat([]ChatMsg{{Role: "user", Content: "test"}}, func(content string) error {
		received = append(received, content)
		return nil
	})

	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if fullText != "Hello world" {
		t.Errorf("fullText = %q, want 'Hello world'", fullText)
	}
	if len(received) != 3 {
		t.Errorf("received %d chunks, want 3", len(received))
	}
}

func TestChatAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"model not found"}}`))
	}))
	defer server.Close()

	cfg := &app.Config{
		Model:    "test-model",
		Endpoint: server.URL,
	}
	client := New(cfg)

	_, err := client.Chat([]ChatMsg{{Role: "user", Content: "test"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code: %v", err)
	}
}

func TestParseStreamMultipleChoices(t *testing.T) {
	// Multiple choices in a single chunk
	resp := ChatResponse{
		Choices: []Choice{
			{Delta: Delta{Content: "Hello"}},
			{Delta: Delta{Content: " world"}},
		},
	}
	data, _ := json.Marshal(resp)
	input := fmt.Sprintf("data: %s\ndata: [DONE]\n", string(data))

	client := New(app.DefaultConfig())
	var received []string
	fullText, err := client.parseStream(strings.NewReader(input), func(content string) error {
		received = append(received, content)
		return nil
	})

	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	if fullText != "Hello world" {
		t.Errorf("fullText = %q, want 'Hello world'", fullText)
	}
	if len(received) != 2 {
		t.Errorf("received %d chunks, want 2", len(received))
	}
}