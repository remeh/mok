package app

import "os"

// Config holds all application configuration.
type Config struct {
	Model               string  `yaml:"model"`
	Endpoint            string  `yaml:"endpoint"`
	BearerToken         string  `yaml:"bearer_token"`
	CWD                 string  `yaml:"cwd"`
	MaxContextTokens    int     `yaml:"max_context_tokens"`
	CompactionThreshold float64 `yaml:"compaction_threshold"`
	KeepRecentTokens    int     `yaml:"keep_recent_tokens"`
	SummarizationModel  string  `yaml:"summarization_model"`
	MaxTokens           int     `yaml:"max_tokens"`
	Debug               bool    `yaml:"debug"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	cwd, _ := os.Getwd()
	return &Config{
		Model:               "qwen3.6-35b-a3b-coder",
		Endpoint:            "http://localhost:8080/v1",
		CWD:                 cwd,
		MaxContextTokens:    131072,
		CompactionThreshold: 0.8,
		KeepRecentTokens:    16384,
		MaxTokens:           0,
	}
}
