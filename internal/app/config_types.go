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
	UILogPath           string  `yaml:"ui_log_path"`

	// Input behavior
	EnableMultiLine      bool `yaml:"enable_multiline"`       // Enable multi-line editing
	EnableAutocomplete   bool `yaml:"enable_autocomplete"`    // Enable command autocomplete
	AutocompleteMaxItems int  `yaml:"autocomplete_max_items"` // Max suggestions to show
	TabCompletes         bool `yaml:"tab_completes"`          // Enable Tab for completion
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
		UILogPath:           "ui.log",

		// Input behavior defaults
		EnableMultiLine:      true,
		EnableAutocomplete:   true,
		AutocompleteMaxItems: 10,
		TabCompletes:         true,
	}
}
