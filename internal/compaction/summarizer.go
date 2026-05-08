package compaction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/user/mok/internal/llm"
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

// Summarize creates a summary of the given messages using a hybrid approach:
// 1. Programmatically extract file operations and key points
// 2. Build a structured summary from extracted data
// 3. Optionally refine with LLM if available
func (s *Summarizer) Summarize(ctx context.Context, messages []llm.Message) (string, error) {
	// Step 1: Programmatically extract file operations
	fileOps := ExtractFileOps(messages)

	// Step 2: Programmatically extract key points
	keyPoints := ExtractKeyPoints(messages)

	// Step 3: Build initial structured summary from extracted data
	initialSummary := BuildSummaryText(
		keyPoints.Goal,
		keyPoints.Context,
		keyPoints.CurrentState,
		keyPoints.NextSteps,
		fileOps,
	)

	// If we have enough context, try to refine with LLM
	if len(messages) > 2 {
		refinedSummary, err := s.refineWithLLM(ctx, messages, initialSummary)
		if err == nil && refinedSummary != "" {
			return refinedSummary, nil
		}
		// Fall back to initial summary if LLM refinement fails
	}

	return initialSummary, nil
}

// refineWithLLM attempts to refine the initial summary using the LLM.
// It provides the initial summary as context and asks the LLM to enhance it.
func (s *Summarizer) refineWithLLM(ctx context.Context, messages []llm.Message, initialSummary string) (string, error) {
	// Create a refined prompt that includes the initial summary
	refinementPrompt := fmt.Sprintf(`You are a context summarization assistant. I have an initial summary of a conversation
that I need you to refine and improve for clarity and completeness.

Initial Summary:
%s

Please improve this summary by:
1. Making it more concise while preserving all important information
2. Ensuring the goal, context, and next steps are clearly articulated
3. Verifying that file operations are accurately listed
4. Improving readability and structure

Output ONLY the improved summary in the same format (with ## sections). Do NOT add any extra text.

Improved Summary:`, initialSummary)

	summaryMsg := llm.Message{
		Role:    "user",
		Content: refinementPrompt,
	}

	req := &llm.ChatRequest{
		Model:     s.model,
		Messages:  []llm.Message{summaryMsg},
		MaxTokens: 2048,
	}

	eventChan, err := s.client.Stream(ctx, req)
	if err != nil {
		return "", err
	}

	var summaryText strings.Builder
	for event := range eventChan {
		if event.Type == "text" {
			summaryText.WriteString(event.Text)
		} else if event.Type == "error" {
			return "", event.Err
		}
	}

	result := strings.TrimSpace(summaryText.String())
	if result == "" {
		return "", fmt.Errorf("empty refinement returned")
	}

	// Extract content from <summary> tags if present
	result = extractSummaryContent(result)

	// Validate that the refined summary has the required sections
	if !strings.Contains(result, "## Goal") || !strings.Contains(result, "## Current State") {
		// If refinement is missing key sections, return the initial summary
		return "", fmt.Errorf("refined summary missing required sections")
	}

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
