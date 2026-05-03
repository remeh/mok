package compaction

import (
	"fmt"
	"strings"
)

// SummarizationPrompt is the prompt template for LLM-driven summarization.
const SummarizationPrompt = `You are a context summarization assistant. Read the conversation below and
produce a structured summary that allows another LLM to continue the work.

Do NOT continue the conversation. ONLY output the structured summary.

Format:
<summary>
## Goal
[What the user is trying to accomplish]

## Context
[Relevant background information]

## Files Read
[List of files read and key findings]

## Files Modified
[List of files edited/written and what changed]

## Current State
[What has been done, what remains, current thinking]
</summary>

Conversation to summarize:
`

// BuildSummarizationPrompt creates a summarization prompt from the messages to summarize.
func BuildSummarizationPrompt(messages []MessageSummary) string {
	var sb strings.Builder
	sb.WriteString(SummarizationPrompt)
	sb.WriteString("\n")

	for _, msg := range messages {
		role := msg.Role
		content := strings.TrimSpace(msg.Content)

		// Truncate long content for the summary prompt
		if len(content) > 2000 {
			content = content[:2000] + "\n...[truncated]"
		}

		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
	}

	return sb.String()
}

// MessageSummary is a simplified message for summarization.
type MessageSummary struct {
	Role    string
	Content string
}

// BuildSummaryText creates the summary text from extracted information.
func BuildSummaryText(goal, context, currentStatus, nextSteps string, fileOps FileOperations) string {
	var sb strings.Builder

	sb.WriteString("## Goal\n")
	if goal != "" {
		sb.WriteString(goal)
	} else {
		sb.WriteString("No clear goal specified.")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Context\n")
	if context != "" {
		sb.WriteString(context)
	} else {
		sb.WriteString("No significant context from previous turns.")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Files Read\n")
	if len(fileOps.ReadFiles) > 0 {
		for _, f := range fileOps.ReadFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	} else {
		sb.WriteString("No files read.")
	}
	sb.WriteString("\n")

	sb.WriteString("## Files Modified\n")
	hasModifications := len(fileOps.WrittenFiles) > 0 || len(fileOps.EditedFiles) > 0
	if hasModifications {
		for _, f := range fileOps.WrittenFiles {
			sb.WriteString(fmt.Sprintf("- %s (written)\n", f))
		}
		for _, f := range fileOps.EditedFiles {
			sb.WriteString(fmt.Sprintf("- %s (edited)\n", f))
		}
	} else {
		sb.WriteString("No files modified.")
	}
	sb.WriteString("\n")

	sb.WriteString("## Current State\n")
	if currentStatus != "" {
		sb.WriteString(currentStatus)
	} else {
		sb.WriteString("No clear current state.")
	}
	sb.WriteString("\n")

	if nextSteps != "" {
		sb.WriteString("\n## Next Steps\n")
		sb.WriteString(nextSteps)
	}

	return sb.String()
}

// CompactMarker is a special message inserted after compaction.
const CompactMarker = "[previous context compacted]"

// BuildCompactMarkerMessage creates a marker message for hard cuts.
func BuildCompactMarkerMessage() string {
	return fmt.Sprintf("%s %d messages were summarized and removed from history.", CompactMarker, 0)
}