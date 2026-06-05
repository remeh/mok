package session

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/user/mok/internal/llm"
	"github.com/user/mok/internal/types"
)

const CurrentVersion = "1"

// SessionMetadata holds session-level information.
type SessionMetadata struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Model     string    `json:"model"`
	Endpoint  string    `json:"endpoint"`
	CWD       string    `json:"cwd"`
}

// SessionConfig holds configuration that should be restored.
type SessionConfig struct {
	MaxContextTokens    int     `json:"max_context_tokens"`
	CompactionThreshold float64 `json:"compaction_threshold"`
	KeepRecentTokens    int     `json:"keep_recent_tokens"`
	MaxTokens           int     `json:"max_tokens"`
	SummarizationModel  string  `json:"summarization_model,omitempty"`
}

// SessionMessage represents a serializable message.
type SessionMessage struct {
	ID               string            `json:"id"`
	Type             types.MessageType `json:"type"`
	Content          string            `json:"content"`
	Summary          string            `json:"summary,omitempty"`
	ThinkingText     string            `json:"thinking_text,omitempty"`
	ToolName         string            `json:"tool_name,omitempty"`
	ToolArgs         string            `json:"tool_args,omitempty"`
	IsError          bool              `json:"is_error"`
	Collapsed        bool              `json:"collapsed"`
	ThinkingExpanded bool              `json:"thinking_expanded"`
	Timestamp        time.Time         `json:"timestamp"`
	IsTurnStats      bool              `json:"is_turn_stats"`
	IsConfirmation   bool              `json:"is_confirmation"`
}

// Session represents the complete session state.
type Session struct {
	Metadata        SessionMetadata    `json:"metadata"`
	Config          SessionConfig      `json:"config"`
	Messages        []SessionMessage   `json:"messages"`
	TokenCount      int                `json:"token_count"`
	HasUserActivity bool               `json:"has_user_activity"`
}

// NewSessionInput holds the minimal config values needed to create a new session.
type NewSessionInput struct {
	Model               string
	Endpoint            string
	CWD                 string
	MaxContextTokens    int
	CompactionThreshold float64
	KeepRecentTokens    int
	MaxTokens           int
	SummarizationModel  string
}

// NewSession creates a new empty session from config values.
func NewSession(input NewSessionInput) *Session {
	now := time.Now()
	return &Session{
		Metadata: SessionMetadata{
			Version:   CurrentVersion,
			CreatedAt: now,
			UpdatedAt: now,
			Model:     input.Model,
			Endpoint:  input.Endpoint,
			CWD:       input.CWD,
		},
		Config: SessionConfig{
			MaxContextTokens:    input.MaxContextTokens,
			CompactionThreshold: input.CompactionThreshold,
			KeepRecentTokens:    input.KeepRecentTokens,
			MaxTokens:           input.MaxTokens,
			SummarizationModel:  input.SummarizationModel,
		},
		Messages: make([]SessionMessage, 0),
	}
}

// AddMessage adds a types.Message to the session.
func (s *Session) AddMessage(msg *types.Message) {
	s.Messages = append(s.Messages, ToSessionMessage(msg))
}

// Save serializes and saves the session to disk at the given path.
func (s *Session) Save(path string) error {
	s.Metadata.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk.
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parsing session file: %w", err)
	}

	// Validate version
	if sess.Metadata.Version == "" {
		sess.Metadata.Version = CurrentVersion
	}

	return &sess, nil
}

// ToAppMessages converts session messages to app Message format.
func (s *Session) ToAppMessages() []*types.Message {
	messages := make([]*types.Message, len(s.Messages))
	for i, sm := range s.Messages {
		messages[i] = ToAppMessage(&sm)
	}
	return messages
}

// ToLLMMessages converts session messages to LLM Message format for
// restoring the agent's conversation history.
// System messages (turn stats, compaction notices, etc.) are skipped.
// Tool call messages are merged into the preceding assistant message,
// and tool results are paired with them in order so their IDs match.
func (s *Session) ToLLMMessages() []llm.Message {
	var result []llm.Message
	tcCounter := 0 // synthetic tool call ID counter

	for i := 0; i < len(s.Messages); i++ {
		msg := &s.Messages[i]

		switch msg.Type {
		case types.MsgUser:
			result = append(result, llm.Message{
				Role:    "user",
				Content: msg.Content,
			})

		case types.MsgAssistant:
			assistant := llm.Message{
				Role:    "assistant",
				Content: msg.Content,
			}

			// Collect tool calls that belong to this assistant turn.
			j := i + 1
			var toolCalls []llm.APIToolCall
			var toolCallIDs []string
			for j < len(s.Messages) && s.Messages[j].Type == types.MsgToolCall {
				tc := &s.Messages[j]
				id := fmt.Sprintf("sess-tc-%d", tcCounter)
				tcCounter++

				toolCalls = append(toolCalls, llm.APIToolCall{
					ID:   id,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.ToolName,
						Arguments: tc.ToolArgs,
					},
				})
				toolCallIDs = append(toolCallIDs, id)
				j++
			}

			if len(toolCalls) > 0 {
				assistant.ToolCalls = toolCalls
			}
			result = append(result, assistant)

			// Pair the following tool results with the tool calls above,
			// in order, so their IDs match.
			for k := 0; k < len(toolCallIDs) && j < len(s.Messages) && s.Messages[j].Type == types.MsgToolResult; k++ {
				tr := &s.Messages[j]
				result = append(result, llm.Message{
					Role:       "tool",
					Content:    tr.Content,
					ToolCallID: toolCallIDs[k],
					Name:       tr.ToolName,
				})
				j++
			}

			i = j - 1 // advance past tool calls and paired results

		case types.MsgToolResult:
			// Orphan tool result with no preceding assistant tool call.
			// Generate a synthetic ID so the message is at least well-formed.
			id := fmt.Sprintf("sess-tc-%d", tcCounter)
			tcCounter++

			result = append(result, llm.Message{
				Role:       "tool",
				Content:    msg.Content,
				ToolCallID: id,
				Name:       msg.ToolName,
			})

		case types.MsgSystem, types.MsgToolCall:
			// Skip UI-only messages (turn stats, compaction notices, tool calls already handled)
			continue
		}
	}

	return result
}

// ToSessionMessage converts a types.Message to a SessionMessage.
func ToSessionMessage(msg *types.Message) SessionMessage {
	return SessionMessage{
		ID:               msg.ID,
		Type:             msg.Type,
		Content:          msg.Content,
		Summary:          msg.Summary,
		ThinkingText:     msg.ThinkingText,
		ToolName:         msg.ToolName,
		ToolArgs:         msg.ToolArgs,
		IsError:          msg.IsError,
		Collapsed:        msg.Collapsed,
		ThinkingExpanded: msg.ThinkingExpanded,
		Timestamp:        msg.Timestamp,
		IsTurnStats:      msg.IsTurnStats,
		IsConfirmation:   msg.IsConfirmation,
	}
}

// ToAppMessage converts a SessionMessage to a types.Message.
func ToAppMessage(sm *SessionMessage) *types.Message {
	msg := &types.Message{
		ID:               sm.ID,
		Type:             sm.Type,
		Content:          sm.Content,
		Summary:          sm.Summary,
		ThinkingText:     sm.ThinkingText,
		ToolName:         sm.ToolName,
		ToolArgs:         sm.ToolArgs,
		IsError:          sm.IsError,
		Collapsed:        sm.Collapsed,
		ThinkingExpanded: sm.ThinkingExpanded,
		Timestamp:        sm.Timestamp,
		IsTurnStats:      sm.IsTurnStats,
		IsConfirmation:   sm.IsConfirmation,
	}
	return msg
}
