package agent

import (
	"context"
	"fmt"

	"github.com/user/mmok/internal/llm"
	"github.com/user/mmok/internal/tools"
)

// AgentConfig holds the configuration values the agent needs.
type AgentConfig struct {
	Model       string
	Temperature float32
	MaxTokens   int
}

// Agent manages the conversation loop.
type Agent struct {
	client       *llm.Client
	config       AgentConfig
	tools        *tools.Registry
	messages     []llm.Message
	tracker      *llm.ContextTracker
	systemPrompt string
	lastThinking string
	quirks       []string
}

// NewAgent creates a new Agent.
func NewAgent(client *llm.Client, cfg AgentConfig, toolRegistry *tools.Registry, quirks []string) *Agent {
	prompt := BuildSystemPrompt(&PromptConfig{})
	return &Agent{
		client:       client,
		config:       cfg,
		tools:        toolRegistry,
		messages:     make([]llm.Message, 0),
		tracker:      llm.NewContextTracker(),
		systemPrompt: prompt,
		quirks:       quirks,
	}
}

// Run starts the agent loop for a single user message.
// Events are sent to the provided channel in real-time.
// The caller controls abort via ctx cancellation.
func (a *Agent) Run(ctx context.Context, userMessage string, events chan<- Event) error {
	return a.runLoop(ctx, userMessage, events)
}

// Messages returns the conversation history.
func (a *Agent) Messages() []llm.Message {
	return a.messages
}

// AddMessage appends a message to history.
func (a *Agent) AddMessage(msg llm.Message) {
	a.messages = append(a.messages, msg)
	a.tracker.AddMessage(msg)
}

// LastThinking returns the thinking text from the last turn.
func (a *Agent) LastThinking() string {
	return a.lastThinking
}

// TokenCount returns the estimated total token count.
func (a *Agent) TokenCount() int {
	return a.tracker.TotalTokens()
}

// HasQuirk returns true if the given model quirk is enabled.
func (a *Agent) HasQuirk(quirk string) bool {
	for _, q := range a.quirks {
		if q == quirk {
			return true
		}
	}
	return false
}

// Tools returns the tool registry.
func (a *Agent) Tools() *tools.Registry {
	return a.tools
}

// String returns a string representation of the agent state.
func (a *Agent) String() string {
	return fmt.Sprintf("Agent{messages: %d, tokens: %d}", len(a.messages), a.tracker.TotalTokens())
}
