package flow

import (
	"time"

	"github.com/user/mok/internal/llm"
)

// AgentRunResult captures the output from a single agent's turn.
type AgentRunResult struct {
	AgentName    string        // name of the agent that ran
	FinalMessage string        // last text-only content from the agent
	Messages     []llm.Message // all messages from this agent's turn
	TokenUsage   *llm.Usage    // accumulated token usage
	Error        error         // non-nil if the agent failed
	StartTime    time.Time     // when the agent started
	EndTime      time.Time     // when the agent finished
}

// Duration returns how long the agent ran.
func (r *AgentRunResult) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// Succeeded returns true if the agent completed without error.
func (r *AgentRunResult) Succeeded() bool {
	return r.Error == nil
}

// HasContent returns true if the agent produced text output.
func (r *AgentRunResult) HasContent() bool {
	return r.FinalMessage != ""
}
