package llm

// EstimateTokens estimates token count from text.
// Uses a simple heuristic: ~4 chars per token for English/text.
// Good enough for compaction thresholds, not for accurate reporting.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len([]rune(text))/4 + 1
}

// ContextTracker tracks total context tokens across messages.
type ContextTracker struct {
	messages []Message
}

// NewContextTracker creates a new ContextTracker.
func NewContextTracker() *ContextTracker {
	return &ContextTracker{
		messages: make([]Message, 0),
	}
}

// TotalTokens returns the estimated total token count.
func (t *ContextTracker) TotalTokens() int {
	total := 0
	for _, msg := range t.messages {
		total += EstimateTokens(msg.Content)
		// Rough estimate for role + structure overhead
		total += 4
	}
	return total
}

// AddMessage appends a message and updates the token count.
func (t *ContextTracker) AddMessage(msg Message) {
	t.messages = append(t.messages, msg)
}

// RemoveMessages removes the first n messages from tracking.
func (t *ContextTracker) RemoveMessages(n int) {
	if n >= len(t.messages) {
		t.messages = make([]Message, 0)
	} else {
		t.messages = t.messages[n:]
	}
}

// Messages returns the tracked messages.
func (t *ContextTracker) Messages() []Message {
	return t.messages
}
