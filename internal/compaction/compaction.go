package compaction

import (
	"context"
	"fmt"
	"time"

	"github.com/user/mmok/internal/llm"
)

// Compactor orchestrates the compaction process.
type Compactor struct {
	client   *llm.Client
	config   CompactionConfig
	summarizer *Summarizer
	model    string // Can use a smaller/cheaper model for summarization
}

// CompactionResult is the result of a compaction operation.
type CompactionResult struct {
	TokensBefore    int
	TokensAfter     int
	MessagesRemoved int
	Summary         string
	CompactSummary  *CompactSummary
	StartTime       time.Time
	EndTime         time.Time
}

// NewCompactor creates a new Compactor.
func NewCompactor(client *llm.Client, config CompactionConfig, model string) *Compactor {
	return &Compactor{
		client:     client,
		config:     config,
		summarizer: NewSummarizer(client, model),
		model:      model,
	}
}

// Compact reduces the message list to fit within the context window.
// Returns the compacted message list and compaction metadata.
// Uses LLM-driven summarization when possible, falls back to hard cut on error.
func (c *Compactor) Compact(ctx context.Context, messages []llm.Message) ([]llm.Message, *CompactionResult, error) {
	startTime := time.Now()

	// Calculate current tokens
	currentTokens := EstimateMessagesTokens(messages)

	// Find the cut point
	cutPoint := FindCutPoint(messages, c.config.MaxContextTokens, c.config.KeepRecentTokens)

	if cutPoint.RemoveBeforeIndex >= len(messages) {
		// No cut needed
		return messages, &CompactionResult{
			TokensBefore:    currentTokens,
			TokensAfter:     currentTokens,
			MessagesRemoved: 0,
			Summary:         "",
			StartTime:       startTime,
			EndTime:         time.Now(),
		}, nil
	}

	// Messages to summarize (from beginning up to cut point, excluding system prompt)
	messagesToSummarize := make([]llm.Message, 0)
	for i := 1; i < cutPoint.RemoveBeforeIndex && i < len(messages); i++ {
		messagesToSummarize = append(messagesToSummarize, messages[i])
	}

	messagesRemoved := len(messagesToSummarize)

	// Try LLM-driven summarization first
	summaryText, err := c.summarizeWithLLM(ctx, messagesToSummarize)
	if err != nil {
		// Fallback to hard cut
		compactMsgs, err := c.hardCut(messages, cutPoint)
		if err != nil {
			return nil, nil, fmt.Errorf("compaction failed and hard cut also failed: %w", err)
		}
		return compactMsgs, &CompactionResult{
			TokensBefore:    currentTokens,
			TokensAfter:     EstimateMessagesTokens(compactMsgs),
			MessagesRemoved: messagesRemoved,
			Summary:         "[hard cut - no summary]",
			StartTime:       startTime,
			EndTime:         time.Now(),
		}, nil
	}

	// Build the compact summary
	compactSummary := &CompactSummary{
		OriginalTokens:  currentTokens,
		SummaryTokens:   llm.EstimateTokens(summaryText),
		MessagesRemoved: messagesRemoved,
		SummaryText:     summaryText,
		Timestamp:       startTime,
	}

	// Build the compacted message list
	compacted, err := c.buildCompactedMessages(messages, cutPoint, compactSummary)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build compacted messages: %w", err)
	}

	return compacted, &CompactionResult{
		TokensBefore:    currentTokens,
		TokensAfter:     EstimateMessagesTokens(compacted),
		MessagesRemoved: messagesRemoved,
		Summary:         summaryText,
		CompactSummary:  compactSummary,
		StartTime:       startTime,
		EndTime:         time.Now(),
	}, nil
}

// summarizeWithLLM uses the LLM to create a summary of the messages.
func (c *Compactor) summarizeWithLLM(ctx context.Context, messages []llm.Message) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages to summarize")
	}

	return c.summarizer.Summarize(ctx, messages)
}

// hardCut removes messages before the cut point without summarization.
func (c *Compactor) hardCut(messages []llm.Message, cutPoint CutPoint) ([]llm.Message, error) {
	if cutPoint.RemoveBeforeIndex >= len(messages) {
		return messages, nil
	}

	// Keep system prompt (index 0) and messages from cut point onward
	compacted := make([]llm.Message, 0, len(messages)-cutPoint.RemoveBeforeIndex+1)
	compacted = append(compacted, messages[0]) // System prompt

	// Add marker message
	compacted = append(compacted, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("[previous context compacted: %d messages removed]", cutPoint.RemoveBeforeIndex-1),
	})

	// Add remaining messages
	compacted = append(compacted, messages[cutPoint.RemoveBeforeIndex:]...)

	return compacted, nil
}

// buildCompactedMessages builds the compacted message list with the summary inserted.
func (c *Compactor) buildCompactedMessages(messages []llm.Message, cutPoint CutPoint, summary *CompactSummary) ([]llm.Message, error) {
	compacted := make([]llm.Message, 0, len(messages)-cutPoint.RemoveBeforeIndex+2)

	// Keep system prompt (index 0)
	compacted = append(compacted, messages[0])

	// Add the compact summary as a system message
	compacted = append(compacted, summary.ToCompactionMessage())

	// Add remaining messages from cut point
	compacted = append(compacted, messages[cutPoint.RemoveBeforeIndex:]...)

	return compacted, nil
}

// ShouldCompact checks if compaction should be triggered for the current context.
func (c *Compactor) ShouldCompact(currentTokens int) bool {
	return ShouldCompact(currentTokens, c.config.MaxContextTokens, c.config.Threshold)
}

// Config returns the compaction configuration.
func (c *Compactor) Config() CompactionConfig {
	return c.config
}