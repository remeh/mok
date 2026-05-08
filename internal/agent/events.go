package agent

import (
	"time"

	"github.com/user/mok/internal/llm"
)

// Event is emitted by the agent loop.
type Event interface {
	eventType() string
}

// EventTurnStart is emitted when a new turn begins.
type EventTurnStart struct {
	StartTime time.Time // When the prompt was submitted
}

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
type EventTurnEnd struct {
	Usage     *llm.Usage // Accumulated usage for the full turn
	Cancelled bool       // true when the user cancelled the turn
	Duration  time.Duration
	EndTime   time.Time // When the turn finished
}

func (EventTurnEnd) eventType() string { return "turn_end" }

// EventError is emitted when an error occurs.
type EventError struct {
	Err error
}

func (EventError) eventType() string { return "error" }

// EventToolCallStart is emitted when a new tool call begins streaming.
type EventToolCallStart struct {
	ToolCallID string
	Name       string
	RawArgs    string
}

func (EventToolCallStart) eventType() string { return "tool_call_start" }

// EventToolCallUpdate is emitted for incremental tool call argument updates.
type EventToolCallUpdate struct {
	ToolCallID string
	RawArgs    string
}

func (EventToolCallUpdate) eventType() string { return "tool_call_update" }

// EventToolCallEnd is emitted when a tool call finishes streaming.
type EventToolCallEnd struct {
	ToolCallID string
	Name       string
	Args       string
}

func (EventToolCallEnd) eventType() string { return "tool_call_end" }

// EventToolResult is emitted after a tool is executed.
type EventToolResult struct {
	ToolCallID string
	Name       string
	Result     string
	IsError    bool
}

func (EventToolResult) eventType() string { return "tool_result" }

// EventCompactionStart is emitted when compaction begins.
type EventCompactionStart struct {
	TokensBefore int
}

func (EventCompactionStart) eventType() string { return "compaction_start" }

// EventCompactionEnd is emitted when compaction completes.
type EventCompactionEnd struct {
	TokensBefore     int
	TokensAfter      int
	MessagesRemoved  int
	SummaryAvailable bool
}

func (EventCompactionEnd) eventType() string { return "compaction_end" }

// EventCompactionError is emitted when compaction fails.
type EventCompactionError struct {
	Err error
}

func (EventCompactionError) eventType() string { return "compaction_error" }
