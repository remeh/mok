package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/user/mmok/internal/llm"
	"github.com/user/mmok/internal/tools"
)

const maxToolCallIterations = 5000

// runLoop executes the agent loop for a single user message.
// Phase 2B: supports tool call → execute → retry cycle.
func (a *Agent) runLoop(ctx context.Context, userMessage string, events chan<- Event) error {
	// Append user message to history
	a.messages = append(a.messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})
	a.tracker.AddMessage(a.messages[len(a.messages)-1])

	events <- EventTurnStart{}

	// Tool call loop: stream → collect → execute tools → repeat (if tool_calls stop)
	iteration := 0
	for {
		if iteration >= maxToolCallIterations {
			err := fmt.Errorf("max tool call iterations (%d) reached", maxToolCallIterations)
			events <- EventError{Err: err}
			events <- EventTurnEnd{}
			return err
		}

		// Build context: system prompt + conversation history
		messages := a.buildContext()

		req := &llm.ChatRequest{
			Model:       a.config.Model,
			Messages:    messages,
			Tools:       a.buildToolSpecs(),
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
		var stopReason string

		// Tool call accumulator
		toolCallMap := make(map[string]*llm.PartialTC)
		var toolCallOrder []*llm.PartialTC

		// Track which tool calls have had start events emitted (for dedup)
		emittedStarts := make(map[string]bool)

		for event := range eventChan {
			switch event.Type {
			case "thinking":
				thinkingText.WriteString(event.ThinkingDelta)
				events <- EventThinkingDelta{Text: event.ThinkingDelta}

			case "text":
				assistantText.WriteString(event.Text)
				events <- EventTextDelta{Text: event.Text}

			case "tool_call":
				isNew, tc, updatedOrder := llm.AccumulateToolCall(toolCallMap, toolCallOrder, event.ToolCall)
				toolCallOrder = updatedOrder
				if isNew {
					events <- EventToolCallStart{
						ToolCallID: tc.ID,
						Name:       tc.Name,
						RawArgs:    tc.RawArgs,
					}
					emittedStarts[tc.ID] = true
				} else {
					// Update the map entry if ID was filled in late
					if tc.ID != "" {
						toolCallMap[tc.ID] = tc
					}
					if !emittedStarts[tc.ID] {
						events <- EventToolCallStart{
							ToolCallID: tc.ID,
							Name:       tc.Name,
							RawArgs:    tc.RawArgs,
						}
						emittedStarts[tc.ID] = true
					} else {
						events <- EventToolCallUpdate{
							ToolCallID: tc.ID,
							RawArgs:    tc.RawArgs,
						}
					}
				}

			case "done":
				usage = event.Usage
				stopReason = event.Stop

			case "error":
				events <- EventError{Err: event.Err}
				events <- EventTurnEnd{}
				return event.Err
			}
		}

		// Save assistant message to history
		assistantMsg := llm.Message{
			Role:    "assistant",
			Content: assistantText.String(),
		}

		// If stop reason is tool_calls, include tool calls and execute them
		if stopReason == "tool_calls" && len(toolCallOrder) > 0 {
			assistantMsg.ToolCalls = llm.ToAPIToolCalls(toolCallOrder)
			a.messages = append(a.messages, assistantMsg)
			a.tracker.AddMessage(assistantMsg)

			// Store thinking text
			a.lastThinking = thinkingText.String()

			// Parse + validate + execute each tool call in order
			for _, tc := range toolCallOrder {
				events <- EventToolCallEnd{
					ToolCallID: tc.ID,
					Name:       tc.Name,
					Args:       tc.RawArgs,
				}

				// Parse accumulated args (three-layer repair)
				args, err := llm.ParseToolArgs(tc.RawArgs)
				if err != nil {
					args = json.RawMessage(`{}`)
				}

				// Validate and coerce against tool schema
				if a.tools != nil {
					if tool := a.tools.Get(tc.Name); tool != nil {
						args = tools.ValidateAndCoerce(tool.Definition().Parameters, args)
					}
				}

				// Execute the tool
				var result string
				var isError bool

				if a.tools != nil {
					if tool := a.tools.Get(tc.Name); tool != nil {
						result, err = tool.Execute(args)
						isError = (err != nil)
						if err != nil {
							result = fmt.Sprintf("Error executing %s: %v", tc.Name, err)
						}
					} else {
						result = fmt.Sprintf("Unknown tool: %s", tc.Name)
						isError = true
					}
				} else {
					result = fmt.Sprintf("No tool registry configured for: %s", tc.Name)
					isError = true
				}

				// Append tool result immediately after in matching order
				toolResultMsg := llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    result,
					Name:       tc.Name,
				}
				a.messages = append(a.messages, toolResultMsg)
				a.tracker.AddMessage(toolResultMsg)

				events <- EventToolResult{
					ToolCallID: tc.ID,
					Name:       tc.Name,
					Result:     result,
					IsError:    isError,
				}
			}

			events <- EventMessageEnd{Usage: usage}

			// Loop back for next LLM call with tool results in context
			iteration++
			continue
		}

		// No tool calls — append text-only assistant message
		a.messages = append(a.messages, assistantMsg)
		a.tracker.AddMessage(assistantMsg)

		// Store thinking text separately so TUI can render it
		a.lastThinking = thinkingText.String()

		events <- EventMessageEnd{Usage: usage}
		events <- EventTurnEnd{}
		return nil
	}
}

// buildToolSpecs returns the tool specs for the current registry (if any).
func (a *Agent) buildToolSpecs() []llm.ToolSpec {
	if a.tools == nil {
		return nil
	}
	return a.tools.ToSpecs()
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
