package agent

import "github.com/user/mmok/internal/llm"

// Event is emitted by the agent loop.
type Event interface {
	eventType() string
}

// EventTurnStart is emitted when a new turn begins.
type EventTurnStart struct{}

func (EventTurnStart) eventType() string { return "turn_start" }

// EventMessageStart is emitted when the assistant begins responding.
type EventMessageStart struct {
	MessageID string
}

func (EventMessageStart) eventType() string { return "message_start" }

// EventTextDelta is emitted for each content text delta.
type EventTextDelta struct {
	Text string
}

func (EventTextDelta) eventType() string { return "text_delta" }

// EventThinkingDelta is emitted for each reasoning/thinking delta.
type EventThinkingDelta struct {
	Text string
}

func (EventThinkingDelta) eventType() string { return "thinking_delta" }

// EventMessageEnd is emitted when the assistant finishes a message.
type EventMessageEnd struct {
	Usage *llm.Usage
}

func (EventMessageEnd) eventType() string { return "message_end" }

// EventTurnEnd is emitted when the turn completes.
type EventTurnEnd struct{}

func (EventTurnEnd) eventType() string { return "turn_end" }

// EventError is emitted when an error occurs.
type EventError struct {
	Err error
}

func (EventError) eventType() string { return "error" }
