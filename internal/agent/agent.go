package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/user/mok/internal/compaction"
	"github.com/user/mok/internal/llm"
	"github.com/user/mok/internal/tools"
)

// AgentConfig holds the configuration values the agent needs.
type AgentConfig struct {
	Model               string
	MaxTokens           int
	CWD                 string
	SystemPrompt        string // Custom system prompt (empty = use default)
	MaxContextTokens    int
	CompactionThreshold float64
	KeepRecentTokens    int
	SummarizationModel  string
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
	debug        *DebugLogger
	compactor    *compaction.Compactor
}

// NewAgent creates a new Agent.
func NewAgent(client *llm.Client, cfg AgentConfig, toolRegistry *tools.Registry, debug *DebugLogger) *Agent {
	var prompt string
	if cfg.SystemPrompt != "" {
		// Use custom system prompt
		prompt = cfg.SystemPrompt
	} else {
		// Build default system prompt
		prompt = BuildSystemPrompt(&PromptConfig{CWD: cfg.CWD, Tools: toolRegistry})
	}
	a := &Agent{
		client:       client,
		config:       cfg,
		tools:        toolRegistry,
		messages:     make([]llm.Message, 0),
		tracker:      llm.NewContextTracker(),
		systemPrompt: prompt,
		debug:        debug,
	}

	// Initialize compactor if compaction is configured
	if cfg.MaxContextTokens > 0 && cfg.CompactionThreshold > 0 {
		compactionConfig := compaction.CompactionConfig{
			MaxContextTokens: cfg.MaxContextTokens,
			Threshold:        cfg.CompactionThreshold,
			KeepRecentTokens: cfg.KeepRecentTokens,
		}
		summarizationModel := cfg.SummarizationModel
		if summarizationModel == "" {
			summarizationModel = cfg.Model // Use same model if not specified
		}
		a.compactor = compaction.NewCompactor(client, compactionConfig, summarizationModel)
		debug.Info("AGENT", "Compaction configured: max_tokens=%d, threshold=%.2f, keep_recent=%d",
			cfg.MaxContextTokens, cfg.CompactionThreshold, cfg.KeepRecentTokens)
	}

	debug.Info("AGENT", "Creating agent with model=%s", cfg.Model)
	if toolRegistry != nil {
		debug.Tool("TOOLS", "Registry initialized with %d tools: %s",
			len(toolRegistry.All()),
			func() string {
				names := make([]string, 0, len(toolRegistry.All()))
				for _, t := range toolRegistry.All() {
					names = append(names, t.Definition().Name)
				}
				return strings.Join(names, ", ")
			}())
	}
	return a
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

// Tools returns the tool registry.
func (a *Agent) Tools() *tools.Registry {
	return a.tools
}

// Debug returns the debug logger.
func (a *Agent) Debug() *DebugLogger {
	return a.debug
}

// SetDebug updates the debug logger.
func (a *Agent) SetDebug(debug *DebugLogger) {
	a.debug = debug
}

// SetClientDebug updates the debug logger on the underlying LLM client.
func (a *Agent) SetClientDebug(debug *DebugLogger) {
	if a.client != nil {
		a.client.WithDebug(debug)
	}
}

// String returns a string representation of the agent state.
func (a *Agent) String() string {
	return fmt.Sprintf("Agent{messages: %d, tokens: %d}", len(a.messages), a.tracker.TotalTokens())
}

// Compact performs compaction on the agent's message history.
// Returns the compaction result or an error.
func (a *Agent) Compact(ctx context.Context) (*compaction.CompactionResult, error) {
	if a.compactor == nil {
		return nil, fmt.Errorf("compaction is not configured")
	}

	a.debug.Info("AGENT", "Compaction triggered")

	// Build full context for compaction
	allMessages := a.buildMessageList()
	currentTokens := a.tracker.TotalTokens()
	a.debug.Info("CONTEXT", "Compacting: %d tokens", currentTokens)

	compacted, result, err := a.compactor.Compact(ctx, allMessages)
	if err != nil {
		a.debug.Event("CONTEXT", "Compaction failed: %v", err)
		return nil, err
	}

	// Update agent's message history with compacted version
	a.messages = compacted[1:] // Skip system prompt (index 0)

	// Reset tracker and re-add messages
	a.tracker = llm.NewContextTracker()
	for _, msg := range compacted {
		a.tracker.AddMessage(msg)
	}

	a.debug.Info("CONTEXT", "Compaction complete: %d → %d tokens, removed %d messages",
		result.TokensBefore, result.TokensAfter, result.MessagesRemoved)

	return result, nil
}

// ManualCompact forces compaction regardless of threshold.
// It reduces history by at least 50% (or to keepRecentTokens if that's higher)
// and summarizes older messages into a compact summary.
func (a *Agent) ManualCompact(ctx context.Context) (*compaction.CompactionResult, error) {
	if a.compactor == nil {
		return nil, fmt.Errorf("compaction is not configured")
	}

	a.debug.Info("AGENT", "Manual compaction triggered")

	// Build full context for compaction
	allMessages := a.buildMessageList()
	currentTokens := a.tracker.TotalTokens()

	// In force mode, always target at least 50% reduction
	targetTokens := currentTokens / 2
	if targetTokens < 1000 {
		targetTokens = 1000
	}

	// But don't go below keepRecentTokens unless we have to
	keepRecent := a.compactor.Config().KeepRecentTokens
	if currentTokens > keepRecent && targetTokens < keepRecent {
		targetTokens = keepRecent
	}

	a.debug.Info("CONTEXT", "Force compacting: %d tokens → target %d (keepRecent=%d)",
		currentTokens, targetTokens, keepRecent)

	// Force compaction
	compacted, result, err := a.compactor.ForceCompact(ctx, allMessages, targetTokens)
	if err != nil {
		a.debug.Event("CONTEXT", "Force compaction failed: %v, using hard cut", err)
		// Fallback: hard cut to targetTokens
		cutPoint := compaction.FindCutPoint(allMessages, targetTokens, keepRecent)
		compacted, err = a.compactor.HardCut(allMessages, cutPoint)
		if err != nil {
			return nil, err
		}
		result = &compaction.CompactionResult{
			TokensBefore:    currentTokens,
			TokensAfter:     compaction.EstimateMessagesTokens(compacted),
			MessagesRemoved: len(allMessages) - len(compacted),
			Summary:         "[hard cut]",
		}
	}

	// Update agent's message history with compacted version
	a.messages = compacted[1:] // Skip system prompt (index 0)

	// Reset tracker and re-add messages
	a.tracker = llm.NewContextTracker()
	for _, msg := range compacted {
		a.tracker.AddMessage(msg)
	}

	a.debug.Info("CONTEXT", "Force compaction complete: %d → %d tokens, removed %d messages",
		result.TokensBefore, result.TokensAfter, result.MessagesRemoved)

	return result, nil
}
