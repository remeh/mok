package flow

// AgentDefinition describes a named, configurable agent.
// Fields are populated from the "agents" section of mok.yaml.
type AgentDefinition struct {
	Name                string  // key in the YAML map (set programmatically, not from YAML)
	Model               string  `yaml:"model"`                // required
	Prompt              string  `yaml:"prompt"`               // role-specific prompt (required)
	Endpoint            string  `yaml:"endpoint"`             // optional, inherits from global
	BearerToken         string  `yaml:"bearer_token"`         // optional, inherits from global
	MaxTokens           int     `yaml:"max_tokens"`           // optional
	MaxContextTokens    int     `yaml:"max_context_tokens"`   // optional
	CompactionThreshold float64 `yaml:"compaction_threshold"` // optional
	KeepRecentTokens    int     `yaml:"keep_recent_tokens"`   // optional
	SummarizationModel  string  `yaml:"summarization_model"`  // optional
}

// FlowDefinition is an ordered sequence of agent names.
type FlowDefinition struct {
	Name  string   `yaml:"name"`  // key in the YAML map (set programmatically, not from YAML)
	Steps []string `yaml:"steps"` // ordered agent names
}
