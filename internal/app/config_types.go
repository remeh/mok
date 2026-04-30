package app

// Config holds all application configuration.
type Config struct {
	Model               string   `yaml:"model"`
	Endpoint            string   `yaml:"endpoint"`
	BearerToken         string   `yaml:"bearer_token"`
	MaxContextTokens    int      `yaml:"max_context_tokens"`
	CompactionThreshold float64  `yaml:"compaction_threshold"`
	KeepRecentTokens    int      `yaml:"keep_recent_tokens"`
	Temperature         float32  `yaml:"temperature"`
	MaxTokens           int      `yaml:"max_tokens"`
	ModelQuirks         []string `yaml:"model_quirks"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Model:               "qwen3.6-35b-a3b-coder",
		Endpoint:            "http://localhost:8080/v1",
		MaxContextTokens:    131072,
		CompactionThreshold: 0.8,
		KeepRecentTokens:    16384,
		Temperature:         0.7,
		MaxTokens:           0,
		ModelQuirks:         []string{},
	}
}

// HasQuirk returns true if the given quirk is enabled.
func (c *Config) HasQuirk(quirk string) bool {
	for _, q := range c.ModelQuirks {
		if q == quirk {
			return true
		}
	}
	return false
}
