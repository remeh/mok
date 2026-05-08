package compaction

import (
	"github.com/user/mok/internal/llm"
)

// CompactionConfig holds the configuration for compaction.
type CompactionConfig struct {
	MaxContextTokens int     // Model context window (e.g., 131072)
	Threshold        float64 // Compact at this fraction (e.g., 0.8 = 80%)
	KeepRecentTokens int     // Always keep this many tokens at the end
}

// DefaultCompactionConfig returns a CompactionConfig with sensible defaults.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		MaxContextTokens: 131072,
		Threshold:        0.8,
		KeepRecentTokens: 16384,
	}
}

// ShouldCompact determines if compaction should be triggered.
func ShouldCompact(currentTokens, maxTokens int, threshold float64) bool {
	if maxTokens <= 0 {
		return false
	}
	if threshold <= 0 {
		return false
	}
	return currentTokens > int(float64(maxTokens)*threshold)
}

// EstimateMessageTokens estimates the token count for a message.
func EstimateMessageTokens(msg llm.Message) int {
	return llm.EstimateTokens(msg.Content) + 4 // Add overhead for role + structure
}

// EstimateMessagesTokens estimates the total token count for a list of messages.
func EstimateMessagesTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateMessageTokens(msg)
	}
	return total
}
