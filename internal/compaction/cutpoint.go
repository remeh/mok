package compaction

import (
	"github.com/user/mok/internal/llm"
)

// CutPoint represents a point in the message history where we can cut.
type CutPoint struct {
	RemoveBeforeIndex int // Remove all messages before this index (0-indexed, after system prompt)
	TokensToRemove    int // Estimated tokens being removed
	TokensRemaining   int // Estimated tokens after removal
}

// FindCutPoint finds the optimal cut point to bring the context under the target.
// It cuts at message boundaries (turn boundaries: user + assistant + tool results).
// Never cuts in the middle of a turn.
// Always keeps at least keepRecentTokens worth of messages at the end.
func FindCutPoint(messages []llm.Message, targetTokens, keepRecentTokens int) CutPoint {
	if len(messages) <= 1 {
		// Only system prompt or nothing, can't cut
		return CutPoint{
			RemoveBeforeIndex: len(messages),
			TokensToRemove:    0,
			TokensRemaining:   EstimateMessagesTokens(messages),
		}
	}

	// Start from the oldest message after system prompt (index 1)
	// Calculate cumulative tokens from the beginning
	totalTokens := EstimateMessagesTokens(messages)

	// If already under target, no cut needed
	if totalTokens <= targetTokens {
		return CutPoint{
			RemoveBeforeIndex: len(messages),
			TokensToRemove:    0,
			TokensRemaining:   totalTokens,
		}
	}

	// Build turn boundaries: each turn is (user, assistant, [tool results])
	// We need to find the earliest turn that, when removed, brings us under target
	turnBoundaries := findTurnBoundaries(messages)

	// Calculate tokens from each turn boundary to the end
	// We want to find the earliest boundary where tokens from that point <= targetTokens
	// but we also need to keep keepRecentTokens worth of messages

	// Calculate cumulative tokens from each position to the end
	tokensFromEnd := make([]int, len(messages)+1)
	tokensFromEnd[len(messages)] = 0
	for i := len(messages) - 1; i >= 0; i-- {
		tokensFromEnd[i] = tokensFromEnd[i+1] + EstimateMessageTokens(messages[i])
	}

	// Find the earliest turn boundary where tokens from that point is under target
	// but we need to ensure we keep enough recent tokens
	bestCut := CutPoint{
		RemoveBeforeIndex: len(messages),
		TokensToRemove:    0,
		TokensRemaining:   totalTokens,
	}

	for _, boundary := range turnBoundaries {
		if boundary == 0 {
			continue // Skip system prompt
		}

		tokensRemaining := tokensFromEnd[boundary]

		// Check if keeping from this boundary would be under target
		if tokensRemaining <= targetTokens {
			tokensToRemove := totalTokens - tokensRemaining

			// Check if we're keeping enough recent tokens
			if tokensRemaining >= keepRecentTokens || boundary >= len(messages)-1 {
				bestCut = CutPoint{
					RemoveBeforeIndex: boundary,
					TokensToRemove:    tokensToRemove,
					TokensRemaining:   tokensRemaining,
				}
				break // Found the earliest valid cut point
			}
		}
	}

	// If no valid cut point found, cut as much as possible while keeping recent tokens
	if bestCut.RemoveBeforeIndex == len(messages) && totalTokens > targetTokens {
		// Find how many messages from the end give us keepRecentTokens
		recentTokens := 0
		cutIndex := len(messages)
		for i := len(messages) - 1; i >= 1; i-- {
			recentTokens += EstimateMessageTokens(messages[i])
			if recentTokens >= keepRecentTokens {
				cutIndex = i
				break
			}
		}

		tokensToRemove := totalTokens - tokensFromEnd[cutIndex]
		bestCut = CutPoint{
			RemoveBeforeIndex: cutIndex,
			TokensToRemove:    tokensToRemove,
			TokensRemaining:   tokensFromEnd[cutIndex],
		}
	}

	return bestCut
}

// findTurnBoundaries returns the indices where turns begin.
// A turn begins with a user message and includes the assistant response and any tool results.
func findTurnBoundaries(messages []llm.Message) []int {
	var boundaries []int

	// Always include index 1 (first message after system prompt) if it exists
	if len(messages) > 1 {
		boundaries = append(boundaries, 1)
	}

	// Find subsequent turn boundaries (user messages that come after assistant messages)
	for i := 2; i < len(messages); i++ {
		if messages[i].Role == "user" && messages[i-1].Role == "assistant" {
			boundaries = append(boundaries, i)
		}
	}

	return boundaries
}

// ApplyCut removes messages before the cut point and returns the compacted list.
func ApplyCut(messages []llm.Message, cutPoint CutPoint) []llm.Message {
	if cutPoint.RemoveBeforeIndex >= len(messages) {
		return messages
	}
	if cutPoint.RemoveBeforeIndex <= 0 {
		// Keep everything (shouldn't happen in practice)
		return messages
	}
	return messages[cutPoint.RemoveBeforeIndex:]
}
