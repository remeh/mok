package app

// Config holds all application configuration.
type Config struct {
	Model               string  `yaml:"model"`
	Endpoint            string  `yaml:"endpoint"`
	APIKey              string  `yaml:"api_key"`
	MaxContextTokens    int     `yaml:"max_context_tokens"`
	CompactionThreshold float64 `yaml:"compaction_threshold"`
	KeepRecentTokens    int     `yaml:"keep_recent_tokens"`
	Temperature         float32 `yaml:"temperature"`
	MaxTokens           int     `yaml:"max_tokens"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Model:               "qwen3-8b",
		Endpoint:            "http://localhost:8080/v1",
		APIKey:              "",
		MaxContextTokens:    131072,
		CompactionThreshold: 0.8,
		KeepRecentTokens:    16384,
		Temperature:         0.0,
		MaxTokens:           0,
	}
}
