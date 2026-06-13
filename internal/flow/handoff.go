package flow

import (
	"fmt"
	"strings"

	"github.com/user/mok/internal/compaction"
	"github.com/user/mok/internal/llm"
)

// BuildHandoffMessage constructs the handoff message that agent N+1 receives
// about what agent N did. It includes a summary of the previous agent's work,
// the original user goal, and instructions for the current agent.
func BuildHandoffMessage(opts HandoffOptions) string {
	var sb strings.Builder

	// Header: who is handing off to whom
	sb.WriteString(fmt.Sprintf("[Handoff from %s (%s)]\n\n", opts.PreviousAgentName, opts.PreviousAgentRole))

	// Summary of previous agent's work
	if opts.Summary != "" {
		sb.WriteString("Here's what was done:\n\n")
		sb.WriteString(opts.Summary)
		sb.WriteString("\n\n")
	}

	// The original user goal (so the next agent remembers the big picture)
	if opts.OriginalGoal != "" {
		sb.WriteString("User's original request:\n")
		sb.WriteString(fmt.Sprintf("%q\n\n", opts.OriginalGoal))
	}

	// Instruction for the current agent
	sb.WriteString(fmt.Sprintf(
		"You are now taking over as %s (%s). Based on the context above, continue the work. "+
			"Use the read, edit, write, and bash tools as needed.\n",
		opts.CurrentAgentName, opts.CurrentAgentRole))

	return sb.String()
}

// HandoffOptions holds the parameters for building a handoff message.
type HandoffOptions struct {
	OriginalGoal      string // the user's original request
	PreviousAgentName string // name of the agent that just finished
	PreviousAgentRole string // role description (prompt) of the previous agent
	Summary           string // summarized output from the previous agent
	CurrentAgentName  string // name of the agent about to start
	CurrentAgentRole  string // role description (prompt) of the current agent
}

// BuildHandoffSummary creates a summary from an agent's output messages
// using programmatic extraction of key points and file operations.
func BuildHandoffSummary(messages []llm.Message) string {
	if len(messages) == 0 {
		return ""
	}

	keyPoints := compaction.ExtractKeyPoints(messages)
	fileOps := compaction.ExtractFileOps(messages)

	return compaction.BuildSummaryText(
		keyPoints.Goal,
		keyPoints.Context,
		keyPoints.CurrentState,
		keyPoints.NextSteps,
		fileOps,
	)
}
