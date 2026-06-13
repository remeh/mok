package flow

import (
	"fmt"

	"github.com/user/mok/internal/agent"
	"github.com/user/mok/internal/llm"
	"github.com/user/mok/internal/tools"
)

// AgentFactory creates configured Agent instances from AgentDefinitions.
// It caches LLM clients keyed by (endpoint, bearerToken) to avoid duplicate
// connections when multiple agents share the same endpoint.
type AgentFactory struct {
	globalConfig *AgentConfigGlobal
	toolRegistry *tools.Registry
	debug        *agent.DebugLogger
	clients      map[string]*llm.Client // keyed by endpoint URL
}

// AgentConfigGlobal holds the global fallback values for agent config resolution.
// Extracted to avoid importing the full app.Config package.
type AgentConfigGlobal struct {
	Model               string
	Endpoint            string
	BearerToken         string
	MaxTokens           int
	MaxContextTokens    int
	CompactionThreshold float64
	KeepRecentTokens    int
	SummarizationModel  string
	CWD                 string
}

// NewAgentFactory creates a new AgentFactory.
func NewAgentFactory(global *AgentConfigGlobal, toolRegistry *tools.Registry, debug *agent.DebugLogger) *AgentFactory {
	return &AgentFactory{
		globalConfig: global,
		toolRegistry: toolRegistry,
		debug:        debug,
		clients:      make(map[string]*llm.Client),
	}
}

// BuildAgent creates a fully configured Agent from an AgentDefinition.
// Returns a new *agent.Agent with the resolved config and system prompt.
func (f *AgentFactory) BuildAgent(def AgentDefinition) (*agent.Agent, error) {
	if def.Name == "" {
		return nil, fmt.Errorf("agent definition has no name")
	}
	if def.Model == "" {
		return nil, fmt.Errorf("agent %q: model is required", def.Name)
	}
	if def.Prompt == "" {
		return nil, fmt.Errorf("agent %q: prompt is required", def.Name)
	}

	// Resolve config values: per-agent → global fallback
	resolved := f.resolveConfig(def)

	// Build the system prompt using PromptPrefix from the agent definition
	systemPrompt := agent.BuildSystemPrompt(&agent.PromptConfig{
		CWD:          f.globalConfig.CWD,
		Tools:        f.toolRegistry,
		PromptPrefix: def.Prompt,
	})

	// This tells NewAgent to use our built prompt instead of the default
	cfg := agent.AgentConfig{
		Model:               resolved.Model,
		MaxTokens:           resolved.MaxTokens,
		CWD:                 f.globalConfig.CWD,
		SystemPrompt:        systemPrompt,
		MaxContextTokens:    resolved.MaxContextTokens,
		CompactionThreshold: resolved.CompactionThreshold,
		KeepRecentTokens:    resolved.KeepRecentTokens,
		SummarizationModel:  resolved.SummarizationModel,
	}

	// Get or create LLM client for this endpoint
	client := f.getClient(resolved.Endpoint, resolved.BearerToken)

	return agent.NewAgent(client, cfg, f.toolRegistry, f.debug), nil
}

// resolveConfig applies the AgentDefinition fields over the global defaults.
type resolvedAgentConfig struct {
	Model               string
	Endpoint            string
	BearerToken         string
	MaxTokens           int
	MaxContextTokens    int
	CompactionThreshold float64
	KeepRecentTokens    int
	SummarizationModel  string
}

func (f *AgentFactory) resolveConfig(def AgentDefinition) resolvedAgentConfig {
	rc := resolvedAgentConfig{
		Model:               f.globalConfig.Model,
		Endpoint:            f.globalConfig.Endpoint,
		BearerToken:         f.globalConfig.BearerToken,
		MaxTokens:           f.globalConfig.MaxTokens,
		MaxContextTokens:    f.globalConfig.MaxContextTokens,
		CompactionThreshold: f.globalConfig.CompactionThreshold,
		KeepRecentTokens:    f.globalConfig.KeepRecentTokens,
		SummarizationModel:  f.globalConfig.SummarizationModel,
	}

	if def.Model != "" {
		rc.Model = def.Model
	}
	if def.Endpoint != "" {
		rc.Endpoint = def.Endpoint
	}
	if def.BearerToken != "" {
		rc.BearerToken = def.BearerToken
	}
	if def.MaxTokens > 0 {
		rc.MaxTokens = def.MaxTokens
	}
	if def.MaxContextTokens > 0 {
		rc.MaxContextTokens = def.MaxContextTokens
	}
	if def.CompactionThreshold > 0 {
		rc.CompactionThreshold = def.CompactionThreshold
	}
	if def.KeepRecentTokens > 0 {
		rc.KeepRecentTokens = def.KeepRecentTokens
	}
	if def.SummarizationModel != "" {
		rc.SummarizationModel = def.SummarizationModel
	}

	return rc
}

// getClient returns a cached LLM client for the given endpoint, creating one if needed.
func (f *AgentFactory) getClient(endpoint, bearerToken string) *llm.Client {
	key := endpoint + "\x00" + bearerToken
	if c, ok := f.clients[key]; ok {
		return c
	}
	c := llm.NewClient(endpoint, bearerToken)
	if f.debug != nil {
		c.WithDebug(f.debug)
	}
	f.clients[key] = c
	return c
}
