package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/user/mmok/internal/llm"
	"github.com/user/mmok/internal/quirks"
	"github.com/user/mmok/internal/tools"
)

const maxToolCallIterations = 5000

// runLoop executes the agent loop for a single user message.
// Phase 2B: supports tool call → execute → retry cycle.
func (a *Agent) runLoop(ctx context.Context, userMessage string, events chan<- Event) error {
	debug := a.debug

	// Append user message to history
	a.messages = append(a.messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})
	a.tracker.AddMessage(a.messages[len(a.messages)-1])

	events <- EventTurnStart{}
	if debug != nil {
		debug.Event("AGENT", "Turn started")
		debug.Event("AGENT", "User message: %q", userMessage)
	}

	// Tool call loop: stream → collect → execute tools → repeat (if tool_calls stop)
	iteration := 0
	emptyRetries := 0
	for {
		if iteration >= maxToolCallIterations {
			err := fmt.Errorf("max tool call iterations (%d) reached", maxToolCallIterations)
			events <- EventError{Err: err}
			events <- EventTurnEnd{}
			if debug != nil {
				debug.Event("AGENT", "Turn ended (max iterations)")
			}
			return err
		}

		if debug != nil {
			debug.Event("AGENT", "Iteration %d", iteration)
		}

		// Build context: system prompt + conversation history
		messages := a.buildContext()
		if debug != nil {
			debug.Request("CONTEXT", "Context built: %d messages, %d tokens", len(messages), a.tracker.TotalTokens())
		}

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
			if debug != nil {
				debug.Response("AGENT", "Stream failed: %v", err)
			}
			return err
		}

		events <- EventMessageStart{}
		if debug != nil {
			debug.Event("STREAM", "Stream started")
		}

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
			if debug != nil {
				debug.Event("STREAM", "Received event: type=%s", event.Type)
			}
			switch event.Type {
			case "thinking":
				thinkingText.WriteString(event.ThinkingDelta)
				events <- EventThinkingDelta{Text: event.ThinkingDelta}
				if debug != nil {
					debug.Event("EVENT", "thinking_delta: %q", truncateDebug(event.ThinkingDelta, 80))
				}

			case "text":
				assistantText.WriteString(event.Text)
				events <- EventTextDelta{Text: event.Text}
				if debug != nil {
					debug.Event("EVENT", "text_delta: %q", truncateDebug(event.Text, 80))
				}

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
					if debug != nil {
						debug.Event("EVENT", "tool_call_start: id=%s name=%s", tc.ID, tc.Name)
					}
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
						if debug != nil {
							debug.Event("EVENT", "tool_call_start (late): id=%s name=%s", tc.ID, tc.Name)
						}
					} else {
						events <- EventToolCallUpdate{
							ToolCallID: tc.ID,
							RawArgs:    tc.RawArgs,
						}
						if debug != nil {
							debug.Event("EVENT", "tool_call_update: id=%s args=%q", tc.ID, truncateDebug(tc.RawArgs, 60))
						}
					}
				}

			case "done":
				usage = event.Usage
				stopReason = event.Stop
				if debug != nil {
					if usage != nil {
						debug.Response("EVENT", "message_end: usage={PromptTokens=%d CompletionTokens=%d}",
							usage.PromptTokens, usage.CompletionTokens)
					}
					debug.Event("EVENT", "stop_reason=%s", stopReason)
				}

			case "error":
				events <- EventError{Err: event.Err}
				events <- EventTurnEnd{}
				if debug != nil {
					debug.Event("EVENT", "error: %v", event.Err)
				}
				return event.Err
			}
		}

		if debug != nil {
			debug.Event("STREAM", "Stream ended, stop_reason=%s, tool_calls=%d, text_len=%d, thinking_len=%d",
				stopReason, len(toolCallOrder), assistantText.Len(), thinkingText.Len())
		}

		// Quirk: some models return stop with no content at all (just EOS token).
		// Retry up to MaxEmptyRetries times before surfacing an error.
		if quirks.IsEmptyResponse(stopReason, assistantText.Len(), thinkingText.Len(), len(toolCallOrder), debug) {
			emptyRetries++
			if emptyRetries <= quirks.MaxEmptyRetries {
				if debug != nil {
					debug.Event("AGENT", "Empty response, retrying (%d/%d)", emptyRetries, quirks.MaxEmptyRetries)
				}
				iteration++
				continue
			}
			err := fmt.Errorf("model returned empty response after %d retries", quirks.MaxEmptyRetries)
			events <- EventError{Err: err}
			events <- EventTurnEnd{}
			if debug != nil {
				debug.Event("AGENT", "Turn ended (empty response after retries)")
			}
			return err
		}

		// Reset empty retries on successful response
		emptyRetries = 0

		// Some models are not sending any content after they're very
		// last thought, let's use the thinking data as content instead.
		// Also, some sanitizing to remove possible leaky tags.
		content, _ := quirks.UseThinkingAsContent(
			assistantText.String(),
			thinkingText.String(),
			debug,
		)
		content, _ = quirks.SanitizeContent(content, debug)

		// Save assistant message to history
		assistantMsg := llm.Message{
			Role:    "assistant",
			Content: content,
		}

		// If stop reason is tool_calls, include tool calls and execute them
		if stopReason == "tool_calls" && len(toolCallOrder) > 0 {
			assistantMsg.ToolCalls = llm.ToAPIToolCalls(toolCallOrder)
			a.messages = append(a.messages, assistantMsg)
			a.tracker.AddMessage(assistantMsg)

			// Store thinking text
			a.lastThinking = thinkingText.String()

			if debug != nil {
				debug.Tool("TOOL", "Executing %d tool call(s)", len(toolCallOrder))
			}

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
					if debug != nil {
						debug.Tool("TOOL", "%s arg parse error (using {}): %v", tc.Name, err)
					}
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

				if debug != nil {
					debug.Tool("TOOL", "Executing %s with args: %s", tc.Name, string(args))
				}

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

				if debug != nil {
					debug.Tool("TOOL", "%s completed: err=%v, output_len=%d", tc.Name, err, len(result))
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
				if debug != nil {
					debug.Event("EVENT", "tool_result: id=%s name=%s success=%v len=%d",
						tc.ID, tc.Name, !isError, len(result))
				}
			}

			events <- EventMessageEnd{Usage: usage}

			// Loop back for next LLM call with tool results in context
			iteration++
			if debug != nil {
				debug.Event("AGENT", "Iteration %d, tool calls executed, continuing loop", iteration)
			}
			continue
		}

		// No tool calls — append text-only assistant message
		a.messages = append(a.messages, assistantMsg)
		a.tracker.AddMessage(assistantMsg)

		// Store thinking text separately so TUI can render it
		a.lastThinking = thinkingText.String()

		events <- EventMessageEnd{Usage: usage}
		events <- EventTurnEnd{}
		if debug != nil {
			debug.Event("AGENT", "Turn completed successfully")
		}
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

	if a.debug != nil {
		debug := a.debug
		debug.Request("CONTEXT", "Building context with %d messages", len(messages))
		for i, msg := range messages {
			preview := truncateDebug(msg.Content, 60)
			tokenEst := llm.EstimateTokens(msg.Content)
			if msg.Role == "system" {
				debug.Request("CONTEXT", "Message %d: system (%d tokens): %s", i, tokenEst, preview)
			} else {
				debug.Request("CONTEXT", "Message %d: %s (%d tokens): %s", i, msg.Role, tokenEst, preview)
			}
		}
		debug.Request("CONTEXT", "Total: %d messages, %d tokens", len(messages), a.tracker.TotalTokens())
	}

	return messages
}

// truncateDebug truncates a string for debug output.
func truncateDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
