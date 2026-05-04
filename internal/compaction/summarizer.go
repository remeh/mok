package compaction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/user/mmok/internal/llm"
)

// Summarizer handles LLM-driven summarization of conversation history.
type Summarizer struct {
	client *llm.Client
	model  string // Can use a smaller/cheaper model for summarization
}

// NewSummarizer creates a new Summarizer.
func NewSummarizer(client *llm.Client, model string) *Summarizer {
	return &Summarizer{
		client: client,
		model:  model,
	}
}

// Summarize creates a summary of the given messages.
func (s *Summarizer) Summarize(ctx context.Context, messages []llm.Message) (string, error) {
	// Convert messages to summary format
	var summaryMessages []MessageSummary
	for _, msg := range messages {
		summaryMessages = append(summaryMessages, MessageSummary{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Build the summarization prompt
	prompt := BuildSummarizationPrompt(summaryMessages)

	// Send to LLM
	summaryMsg := llm.Message{
		Role:    "user",
		Content: prompt,
	}

	req := &llm.ChatRequest{
		Model:     s.model,
		Messages:  []llm.Message{summaryMsg},
		MaxTokens: 2048, // Reasonable size for a summary
	}

	// Stream the response and collect it
	eventChan, err := s.client.Stream(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create summary stream: %w", err)
	}

	var summaryText strings.Builder
	for event := range eventChan {
		if event.Type == "text" {
			summaryText.WriteString(event.Text)
		} else if event.Type == "error" {
			return "", fmt.Errorf("summary stream error: %w", event.Err)
		}
	}

	result := strings.TrimSpace(summaryText.String())
	if result == "" {
		return "", fmt.Errorf("empty summary returned")
	}

	// Extract content from <summary> tags if present
	result = extractSummaryContent(result)

	return result, nil
}

// extractSummaryContent extracts the content from <summary> tags.
func extractSummaryContent(text string) string {
	// Look for <summary>...</summary>
	startTag := "<summary>"
	endTag := "</summary>"

	startIdx := strings.Index(text, startTag)
	if startIdx != -1 {
		startIdx += len(startTag)
		endIdx := strings.Index(text[startIdx:], endTag)
		if endIdx != -1 {
			return strings.TrimSpace(text[startIdx : startIdx+endIdx])
		}
	}

	// If no tags, return the whole text
	return strings.TrimSpace(text)
}

// ParseSummary parses a summary text and extracts structured information.
func ParseSummary(summary string) (goal, context, filesRead, filesModified, currentState string, err error) {
	lines := strings.Split(summary, "\n")
	var currentSection string
	var sectionContent strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for section headers
		if strings.HasPrefix(trimmed, "## ") {
			// Save previous section
			if currentSection != "" {
				saveSection(currentSection, sectionContent.String(), &goal, &context, &filesRead, &filesModified, &currentState)
			}
			// Start new section
			currentSection = strings.TrimPrefix(trimmed, "## ")
			sectionContent.Reset()
		} else {
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != "" {
		saveSection(currentSection, sectionContent.String(), &goal, &context, &filesRead, &filesModified, &currentState)
	}

	return goal, context, filesRead, filesModified, currentState, nil
}

func saveSection(section, content string, goal, context, filesRead, filesModified, currentState *string) {
	content = strings.TrimSpace(content)
	switch section {
	case "Goal":
		*goal = content
	case "Context":
		*context = content
	case "Files Read":
		*filesRead = content
	case "Files Modified":
		*filesModified = content
	case "Current State":
		*currentState = content
	}
}

// CompactSummary represents the result of a compaction operation.
type CompactSummary struct {
	OriginalTokens  int       // Tokens before compaction
	SummaryTokens   int       // Tokens in the summary
	MessagesRemoved int       // Number of messages summarized
	SummaryText     string    // The actual summary
	Timestamp       time.Time // When compaction occurred
}

// ToCompactionMessage converts the summary to a message that can be inserted into history.
func (cs *CompactSummary) ToCompactionMessage() llm.Message {
	return llm.Message{
		Role:    "system",
		Content: cs.SummaryText,
	}
}

// FromCompactionMessage extracts a CompactSummary from a system message.
func FromCompactionMessage(msg llm.Message) (*CompactSummary, error) {
	if msg.Role != "system" {
		return nil, fmt.Errorf("not a system message")
	}

	// Try to parse as JSON first
	var summary CompactSummary
	if err := json.Unmarshal([]byte(msg.Content), &summary); err == nil {
		return &summary, nil
	}

	// Fallback: treat content as the summary text
	return &CompactSummary{
		SummaryText: msg.Content,
		Timestamp:   time.Now(),
	}, nil
}
