package agent

import (
	"context"
	"strings"

	"github.com/user/mmok/internal/llm"
)

// runLoop executes the agent loop for a single user message.
// Phase 2A: text-only streaming (no tool calls).
func (a *Agent) runLoop(ctx context.Context, userMessage string, events chan<- Event) error {
	// Append user message to history
	a.messages = append(a.messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})
	a.tracker.AddMessage(a.messages[len(a.messages)-1])

	events <- EventTurnStart{}

	// Build context: system prompt + conversation history
	messages := a.buildContext()

	req := &llm.ChatRequest{
		Model:       a.config.Model,
		Messages:    messages,
		Temperature: a.config.Temperature,
		MaxTokens:   a.config.MaxTokens,
	}

	eventChan, err := a.client.Stream(ctx, req)
	if err != nil {
		events <- EventError{Err: err}
		events <- EventTurnEnd{}
		return err
	}

	events <- EventMessageStart{}

	var assistantText strings.Builder
	var thinkingText strings.Builder
	var usage *llm.Usage

	for event := range eventChan {
		switch event.Type {
		case "thinking":
			thinkingText.WriteString(event.ThinkingDelta)
			events <- EventThinkingDelta{Text: event.ThinkingDelta}

		case "text":
			assistantText.WriteString(event.Text)
			events <- EventTextDelta{Text: event.Text}

		case "done":
			usage = event.Usage

		case "error":
			events <- EventError{Err: event.Err}
			events <- EventTurnEnd{}
			return event.Err
		}
	}

	// Save assistant message to history (thinking is excluded)
	assistantMsg := llm.Message{
		Role:    "assistant",
		Content: assistantText.String(),
	}
	a.messages = append(a.messages, assistantMsg)
	a.tracker.AddMessage(assistantMsg)

	// Store thinking text separately so TUI can render it
	a.lastThinking = thinkingText.String()

	events <- EventMessageEnd{Usage: usage}
	events <- EventTurnEnd{}
	return nil
}

// buildContext returns the full message list: system prompt + history.
func (a *Agent) buildContext() []llm.Message {
	messages := make([]llm.Message, 0, len(a.messages)+1)

	// System prompt
	messages = append(messages, llm.Message{
		Role:    "system",
		Content: a.systemPrompt,
	})

	// Conversation history
	messages = append(messages, a.messages...)

	return messages
}
