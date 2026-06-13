package flow

// AgentConfig resolves an AgentDefinition against global defaults.
// Zero/false/empty values from the definition fall back to the global values.
type AgentConfig struct {
	Definition          AgentDefinition
	GlobalModel         string
	GlobalEndpoint      string
	GlobalMaxTokens     int
	GlobalMaxContextTokens    int
	GlobalCompactionThreshold float64
	GlobalKeepRecentTokens    int
}

// ResolvedModel returns the effective model for this agent.
func (c AgentConfig) ResolvedModel() string {
	if c.Definition.Model != "" {
		return c.Definition.Model
	}
	return c.GlobalModel
}

// ResolvedEndpoint returns the effective endpoint.
func (c AgentConfig) ResolvedEndpoint() string {
	if c.Definition.Endpoint != "" {
		return c.Definition.Endpoint
	}
	return c.GlobalEndpoint
}

// ResolvedMaxTokens returns the effective max_tokens.
func (c AgentConfig) ResolvedMaxTokens() int {
	if c.Definition.MaxTokens > 0 {
		return c.Definition.MaxTokens
	}
	return c.GlobalMaxTokens
}

// ResolvedMaxContextTokens returns the effective max_context_tokens.
func (c AgentConfig) ResolvedMaxContextTokens() int {
	if c.Definition.MaxContextTokens > 0 {
		return c.Definition.MaxContextTokens
	}
	return c.GlobalMaxContextTokens
}

// ResolvedCompactionThreshold returns the effective compaction_threshold.
func (c AgentConfig) ResolvedCompactionThreshold() float64 {
	if c.Definition.CompactionThreshold > 0 {
		return c.Definition.CompactionThreshold
	}
	return c.GlobalCompactionThreshold
}

// ResolvedKeepRecentTokens returns the effective keep_recent_tokens.
func (c AgentConfig) ResolvedKeepRecentTokens() int {
	if c.Definition.KeepRecentTokens > 0 {
		return c.Definition.KeepRecentTokens
	}
	return c.GlobalKeepRecentTokens
}
