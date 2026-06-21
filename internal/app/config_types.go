package app

import (
	"os"

	"github.com/user/mok/internal/flow"
)

// Config holds all application configuration.
type Config struct {
	Model               string  `yaml:"model"`
	Endpoint            string  `yaml:"endpoint"`
	BearerToken         string  `yaml:"bearer_token"`
	CWD                 string  `yaml:"cwd"`
	SystemPrompt        string  `yaml:"system_prompt"` // Custom system prompt for one-shot runs
	MaxContextTokens    int     `yaml:"max_context_tokens"`
	CompactionThreshold float64 `yaml:"compaction_threshold"`
	KeepRecentTokens    int     `yaml:"keep_recent_tokens"`
	SummarizationModel  string  `yaml:"summarization_model"`
	MaxTokens           int     `yaml:"max_tokens"`
	Debug               bool    `yaml:"debug"`
	UILogPath           string  `yaml:"ui_log_path"`

	// Bash confirmation
	BashConfirmPolicy    string   `yaml:"bash_confirm_policy"`     // "blocklist", "allowlist", or "none"
	BashConfirmBlocklist []string `yaml:"bash_confirm_blocklist"`  // Dangerous patterns (for blocklist policy)
	BashConfirmAllowlist []string `yaml:"bash_confirm_allowlist"`  // Safe patterns (for allowlist policy)

	// Input behavior
	EnableMultiLine      bool `yaml:"enable_multiline"`       // Enable multi-line editing
	EnableAutocomplete   bool `yaml:"enable_autocomplete"`    // Enable command autocomplete
	AutocompleteMaxItems int  `yaml:"autocomplete_max_items"` // Max suggestions to show
	TabCompletes         bool `yaml:"tab_completes"`          // Enable Tab for completion

	// Multi-agent flow
	Agents      map[string]flow.AgentDefinition `yaml:"agents"`       // nil = single-agent mode
	Flows       map[string][]string             `yaml:"flows"`        // nil = no flows (value is ordered agent names)
	DefaultFlow string                          `yaml:"default_flow"`  // default flow when none specified

	// Config validation warnings (populated during LoadConfig)
	ValidationWarnings []string `yaml:"-"`
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
		UILogPath:           "",

		// Bash confirmation defaults
	BashConfirmPolicy:    "blocklist",
	BashConfirmBlocklist: []string{
		// Destructive commands
		"rm ", "rm -rf", "rm -fr",
		"sudo ", "su ", "su -",
		"chmod ", "chown ",
		"dd ", "mkfs ", "fdisk ", "parted ",
		"> /dev/", ">> /dev/", ">/dev", ">>/dev",
		"eval ", "exec ",
		"python ", "python3 ", "node ", "perl ", "ruby ",
		// I don't want remote call without knowing about them
		"curl ", "wget ",

		// Dangerous flags on otherwise-safe commands (two-pass parameter scan)
		" -delete",               // find ... -delete
		"-exec rm ",              // find ... -exec rm ...
		"-exec rm -rf ",          // find ... -exec rm -rf ...
		"| xargs rm ",            // piped deletion
		"| xargs rm -rf ",
		"| bash",                 // pipe to execution
		"| sh",
	},
	BashConfirmAllowlist: []string{
		"git ", "ls ", "cat ", "grep ", "find ", "which ", "pwd",
		"echo ", "date ", "whoami ", "hostname ",
		"cd ", "mkdir ", "touch ",
		"make ", "go ", "cargo ", "npm ", "pip ",
		"docker ps", "docker images", "docker logs",
	},

	// Input behavior defaults
	EnableMultiLine:      true,
	EnableAutocomplete:   true,
	AutocompleteMaxItems: 10,
	TabCompletes:         true,
}
}
