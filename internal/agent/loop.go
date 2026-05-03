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
	debug.Event("AGENT", "Turn started")
	debug.Event("AGENT", "User message: %q", userMessage)

	// Tool call loop: stream → collect → execute tools → repeat (if tool_calls stop)
	iteration := 0
	emptyRetries := 0
	for {
		if iteration >= maxToolCallIterations {
			err := fmt.Errorf("max tool call iterations (%d) reached", maxToolCallIterations)
			events <- EventError{Err: err}
			events <- EventTurnEnd{}
			debug.Event("AGENT", "Turn ended (max iterations)")
			return err
		}

		debug.Event("AGENT", "Iteration %d", iteration)

		// Build context: system prompt + conversation history
		messages := a.buildContext()
		debug.Request("CONTEXT", "Context built: %d messages, %d tokens", len(messages), a.tracker.TotalTokens())

		req := &llm.ChatRequest{
			Model:     a.config.Model,
			Messages:  messages,
			Tools:     a.buildToolSpecs(),
			MaxTokens: a.config.MaxTokens,
		}

		eventChan, err := a.client.Stream(ctx, req)
		if err != nil {
			events <- EventError{Err: err}
			events <- EventTurnEnd{}
			debug.Response("AGENT", "Stream failed: %v", err)
			return err
		}

		events <- EventMessageStart{}
		debug.Event("STREAM", "Stream started")

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
			debug.Event("STREAM", "Received event: type=%s", event.Type)
			switch event.Type {
			case "thinking":
				thinkingText.WriteString(event.ThinkingDelta)
				events <- EventThinkingDelta{Text: event.ThinkingDelta}
				debug.Event("EVENT", "thinking_delta: %q", truncateDebug(event.ThinkingDelta, 80))

			case "text":
				assistantText.WriteString(event.Text)
				events <- EventTextDelta{Text: event.Text}
				debug.Event("EVENT", "text_delta: %q", truncateDebug(event.Text, 80))

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
					debug.Event("EVENT", "tool_call_start: id=%s name=%s", tc.ID, tc.Name)
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
						debug.Event("EVENT", "tool_call_start (late): id=%s name=%s", tc.ID, tc.Name)
					} else {
						events <- EventToolCallUpdate{
							ToolCallID: tc.ID,
							RawArgs:    tc.RawArgs,
						}
						debug.Event("EVENT", "tool_call_update: id=%s args=%q", tc.ID, truncateDebug(tc.RawArgs, 60))
					}
				}

			case "done":
				usage = event.Usage
				stopReason = event.Stop
				if usage != nil {
					debug.Response("EVENT", "message_end: usage={PromptTokens=%d CompletionTokens=%d}",
						usage.PromptTokens, usage.CompletionTokens)
				}
				debug.Event("EVENT", "stop_reason=%s", stopReason)

			case "error":
				events <- EventError{Err: event.Err}
				events <- EventTurnEnd{}
				debug.Event("EVENT", "error: %v", event.Err)
				return event.Err
			}
		}

		debug.Event("STREAM", "Stream ended, stop_reason=%s, tool_calls=%d, text_len=%d, thinking_len=%d",
			stopReason, len(toolCallOrder), assistantText.Len(), thinkingText.Len())

		// Quirk: some models return stop with no content at all (just EOS token).
		// Retry up to MaxEmptyRetries times before surfacing an error.
		if quirks.IsEmptyResponse(stopReason, assistantText.Len(), thinkingText.Len(), len(toolCallOrder), debug) {
			emptyRetries++
			if emptyRetries <= quirks.MaxEmptyRetries {
				debug.Event("AGENT", "Empty response, retrying (%d/%d)", emptyRetries, quirks.MaxEmptyRetries)
				iteration++
				continue
			}
			err := fmt.Errorf("model returned empty response after %d retries", quirks.MaxEmptyRetries)
			events <- EventError{Err: err}
			events <- EventTurnEnd{}
			debug.Event("AGENT", "Turn ended (empty response after retries)")
			return err
		}

		// Reset empty retries on successful response
		emptyRetries = 0

		// Quirk: some models (e.g. Qwen) emit XML-style tool calls in
		// thinking/content instead of proper JSON tool_calls.
		// Only scan when the standard mechanism produced nothing.
		if len(toolCallOrder) == 0 && (thinkingText.Len() > 0 || assistantText.Len() > 0) {
			var xmlTCs []quirks.QwenXMLToolCall
			var found bool

			// Check thinking text first (XML tool calls often appear here)
			if thinkingText.Len() > 0 {
				xmlTCs, found = quirks.ExtractXMLToolCalls(thinkingText.String(), debug)
			}

			// If not found in thinking, check content
			if !found && assistantText.Len() > 0 {
				xmlTCs, found = quirks.ExtractXMLToolCalls(assistantText.String(), debug)
			}

			if found && len(xmlTCs) > 0 {
				for i, xmlTC := range xmlTCs {
					tc := &llm.PartialTC{
						ID:      fmt.Sprintf("xml-call-%d", i),
						Name:    xmlTC.Name,
						RawArgs: quirks.XMLToolCallArgsToJSON(xmlTC.Args),
					}
					toolCallOrder = append(toolCallOrder, tc)

					events <- EventToolCallStart{
						ToolCallID: tc.ID,
						Name:       tc.Name,
						RawArgs:    tc.RawArgs,
					}

					debug.Event("QUIRK", "xml-tool-call: converted %s with args %s",
						tc.Name, tc.RawArgs)
				}
			}
		}

		// Some models are not sending any content after they're very
		// last thought, let's use the thinking data as content instead.
		// Also, some sanitizing to remove possible leaky tags.
		content, usedThinking := quirks.UseThinkingAsContent(
			assistantText.String(),
			thinkingText.String(),
			debug,
		)
		// SanitizeContent also strips XML tool call markup from the content.
		// This is safe because ExtractXMLToolCalls has already parsed them
		// into toolCallOrder above.
		content, _ = quirks.SanitizeContent(content, debug)

		// If thinking was promoted to content, emit it as a text delta
		// so the TUI renders it as visible content (not just collapsed thinking).
		if usedThinking {
			events <- EventTextDelta{Text: content}
		}

		// Save assistant message to history
		assistantMsg := llm.Message{
			Role:    "assistant",
			Content: content,
		}

		// If we have tool calls (from JSON or XML quirk), execute them
		if len(toolCallOrder) > 0 {
			// Quirk: sanitize tool call args before storing in history.
			// Malformed JSON in history causes server-side 500 on replay.
			sanitized, retryNotice := quirks.SanitizeToolCalls(toolCallOrder, debug)
			toolCallOrder = sanitized.Valid

			if retryNotice != "" {
				// Some tool calls had unrepairable args: store the assistant
				// message (without tool calls) and inject a notice so the
				// LLM knows to retry.
				a.messages = append(a.messages, assistantMsg)
				a.tracker.AddMessage(assistantMsg)
				a.messages = append(a.messages, llm.Message{
					Role:    "user",
					Content: retryNotice,
				})

				// If all tool calls were invalid, skip execution entirely
				if len(toolCallOrder) == 0 {
					events <- EventMessageEnd{Usage: usage}
					iteration++
					continue
				}
			}

			assistantMsg.ToolCalls = llm.ToAPIToolCalls(toolCallOrder)
			if retryNotice == "" {
				a.messages = append(a.messages, assistantMsg)
				a.tracker.AddMessage(assistantMsg)
			}

			// Store thinking text
			a.lastThinking = thinkingText.String()

			debug.Tool("TOOL", "Executing %d tool call(s)", len(toolCallOrder))

			// Execute each tool call in order (args already sanitized)
			for _, tc := range toolCallOrder {
				events <- EventToolCallEnd{
					ToolCallID: tc.ID,
					Name:       tc.Name,
					Args:       tc.RawArgs,
				}

				args := json.RawMessage(tc.RawArgs)

				// Validate and coerce against tool schema
				if a.tools != nil {
					if tool := a.tools.Get(tc.Name); tool != nil {
						args = tools.ValidateAndCoerce(tool.Definition().Parameters, args)
					}
				}

				// Execute the tool
				var result string
				var isError bool

				debug.Tool("TOOL", "Executing %s with args: %s", tc.Name, string(args))

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

				debug.Tool("TOOL", "%s completed: err=%v, output_len=%d", tc.Name, err, len(result))

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
				debug.Event("EVENT", "tool_result: id=%s name=%s success=%v len=%d",
					tc.ID, tc.Name, !isError, len(result))
			}

			events <- EventMessageEnd{Usage: usage}

			// Loop back for next LLM call with tool results in context
			iteration++
			debug.Event("AGENT", "Iteration %d, tool calls executed, continuing loop", iteration)
			continue
		}

		// No tool calls — append text-only assistant message
		a.messages = append(a.messages, assistantMsg)
		a.tracker.AddMessage(assistantMsg)

		// Store thinking text separately so TUI can render it
		a.lastThinking = thinkingText.String()

		events <- EventMessageEnd{Usage: usage}
		events <- EventTurnEnd{}
		debug.Event("AGENT", "Turn completed successfully")
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

	a.debug.Request("CONTEXT", "Building context with %d messages", len(messages))
	for i, msg := range messages {
		preview := truncateDebug(msg.Content, 60)
		tokenEst := llm.EstimateTokens(msg.Content)
		if msg.Role == "system" {
			a.debug.Request("CONTEXT", "Message %d: system (%d tokens): %s", i, tokenEst, preview)
		} else {
			a.debug.Request("CONTEXT", "Message %d: %s (%d tokens): %s", i, msg.Role, tokenEst, preview)
		}
	}
	a.debug.Request("CONTEXT", "Total: %d messages, %d tokens", len(messages), a.tracker.TotalTokens())

	return messages
}

// truncateDebug truncates a string for debug output.
func truncateDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
