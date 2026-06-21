package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/user/mok/internal/flow"
	"gopkg.in/yaml.v3"
)

// configEnvPrefix for environment variables.
const configEnvPrefix = "MOK_"

// LoadConfig reads config with precedence: defaults → file → env → flags.
func LoadConfig(flags map[string]string) (*Config, error) {
	cfg := DefaultConfig()

	// 1. Load from YAML file (search known locations)
	if fileCfg, err := loadFromFile(); err == nil {
		mergeConfig(cfg, fileCfg)
	}

	// 2. Load from environment variables
	if err := loadFromEnv(cfg); err != nil {
		return nil, fmt.Errorf("loading env config: %w", err)
	}

	// 3. Override with CLI flags (highest precedence)
	applyFlags(cfg, flags)

	// 4. Validate agent/flow configuration if present
	// Note: This may add warnings to cfg.ValidationWarnings but doesn't fail
	if err := cfg.validateAgentsAndFlows(); err != nil {
		return nil, fmt.Errorf("validating agents/flows: %w", err)
	}

	return cfg, nil
}

// loadFromFile searches for config.yaml in known locations.
func loadFromFile() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	paths := []string{
		"./mok.yaml",
		"./config.yaml",
		filepath.Join(home, ".config", "mok", "config.yaml"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading config %s: %w", path, err)
		}

		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
		return &cfg, nil
	}

	return nil, fmt.Errorf("no config file found")
}

// loadFromEnv reads MMOK_* environment variables.
func loadFromEnv(cfg *Config) error {
	// Use env struct tags for clean parsing
	type envConfig struct {
		Model               string  `env:"MODEL" envDefault:""`
		Endpoint            string  `env:"ENDPOINT" envDefault:""`
		MaxContextTokens    int     `env:"MAX_CONTEXT_TOKENS" envDefault:"0"`
		CompactionThreshold float64 `env:"COMPACTION_THRESHOLD" envDefault:"0"`
		KeepRecentTokens    int     `env:"KEEP_RECENT_TOKENS" envDefault:"0"`
		MaxTokens           int     `env:"MAX_TOKENS" envDefault:"0"`
	}

	// Build a temporary env map with MMOK_ prefix stripped
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(kv[0], configEnvPrefix) {
			key := strings.TrimPrefix(kv[0], configEnvPrefix)
			envMap[key] = kv[1]
		}
	}

	// Parse manually to avoid needing a full env struct unmarshal
	if v, ok := envMap["MODEL"]; ok && v != "" {
		cfg.Model = v
	}
	if v, ok := envMap["ENDPOINT"]; ok && v != "" {
		cfg.Endpoint = v
	}
	if v, ok := envMap["MAX_CONTEXT_TOKENS"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxContextTokens = n
		}
	}
	if v, ok := envMap["COMPACTION_THRESHOLD"]; ok && v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			cfg.CompactionThreshold = n
		}
	}
	if v, ok := envMap["KEEP_RECENT_TOKENS"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.KeepRecentTokens = n
		}
	}
	if v, ok := envMap["MAX_TOKENS"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxTokens = n
		}
	}
	if v, ok := envMap["BEARER_TOKEN"]; ok && v != "" {
		cfg.BearerToken = v
	}
	if v, ok := envMap["SYSTEM_PROMPT"]; ok && v != "" {
		cfg.SystemPrompt = v
	}
	if v, ok := envMap["DEBUG"]; ok && v == "true" {
		cfg.Debug = true
	}
	if v, ok := envMap["UI_LOG_PATH"]; ok && v != "" {
		cfg.UILogPath = v
	}
	if v, ok := envMap["BASH_CONFIRM_POLICY"]; ok && v != "" {
		cfg.BashConfirmPolicy = v
	}
	if v, ok := envMap["ENABLE_MULTILINE"]; ok && v == "true" {
		cfg.EnableMultiLine = true
	}
	if v, ok := envMap["ENABLE_AUTOCOMPLETE"]; ok && v == "true" {
		cfg.EnableAutocomplete = true
	}
	if v, ok := envMap["AUTOCOMPLETE_MAX_ITEMS"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.AutocompleteMaxItems = n
		}
	}
	if v, ok := envMap["TAB_COMPLETES"]; ok && v == "true" {
		cfg.TabCompletes = true
	}

	return nil
}

// applyFlags applies CLI flag overrides.
func applyFlags(cfg *Config, flags map[string]string) {
	if v, ok := flags["model"]; ok && v != "" {
		cfg.Model = v
	}
	if v, ok := flags["endpoint"]; ok && v != "" {
		cfg.Endpoint = v
	}
	if v, ok := flags["bearer-token"]; ok && v != "" {
		cfg.BearerToken = v
	}
	if v, ok := flags["system-prompt"]; ok && v != "" {
		cfg.SystemPrompt = v
	}
	if v, ok := flags["max-context-tokens"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxContextTokens = n
		}
	}
	if v, ok := flags["max-tokens"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxTokens = n
		}
	}
	if v, ok := flags["debug"]; ok && v == "true" {
		cfg.Debug = true
	}
	if v, ok := flags["ui-log-path"]; ok && v != "" {
		cfg.UILogPath = v
	}
	if v, ok := flags["enable-multiline"]; ok && v == "true" {
		cfg.EnableMultiLine = true
	}
	if v, ok := flags["enable-autocomplete"]; ok && v == "true" {
		cfg.EnableAutocomplete = true
	}
	if v, ok := flags["autocomplete-max-items"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.AutocompleteMaxItems = n
		}
	}
	if v, ok := flags["tab-completes"]; ok && v == "true" {
		cfg.TabCompletes = true
	}
	if v, ok := flags["bash-confirm-policy"]; ok && v != "" {
		cfg.BashConfirmPolicy = v
	}
}

// mergeConfig overlays non-zero values from src onto dst.
func mergeConfig(dst, src *Config) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Endpoint != "" {
		dst.Endpoint = src.Endpoint
	}
	if src.BearerToken != "" {
		dst.BearerToken = src.BearerToken
	}
	if src.SystemPrompt != "" {
		dst.SystemPrompt = src.SystemPrompt
	}
	if src.MaxContextTokens > 0 {
		dst.MaxContextTokens = src.MaxContextTokens
	}
	if src.CompactionThreshold > 0 {
		dst.CompactionThreshold = src.CompactionThreshold
	}
	if src.KeepRecentTokens > 0 {
		dst.KeepRecentTokens = src.KeepRecentTokens
	}
	if src.MaxTokens > 0 {
		dst.MaxTokens = src.MaxTokens
	}
	if src.Debug {
		dst.Debug = true
	}
	if src.BashConfirmPolicy != "" {
		dst.BashConfirmPolicy = src.BashConfirmPolicy
	}
	if len(src.BashConfirmBlocklist) > 0 {
		dst.BashConfirmBlocklist = src.BashConfirmBlocklist
	}
	if len(src.BashConfirmAllowlist) > 0 {
		dst.BashConfirmAllowlist = src.BashConfirmAllowlist
	}
	if src.UILogPath != "" {
		dst.UILogPath = src.UILogPath
	}
	if src.EnableMultiLine {
		dst.EnableMultiLine = src.EnableMultiLine
	}
	if src.EnableAutocomplete {
		dst.EnableAutocomplete = src.EnableAutocomplete
	}
	if src.AutocompleteMaxItems > 0 {
		dst.AutocompleteMaxItems = src.AutocompleteMaxItems
	}
	if src.TabCompletes {
		dst.TabCompletes = src.TabCompletes
	}
	// Merge agent/flow maps (full replacement, not field-level merge)
	if len(src.Agents) > 0 {
		dst.Agents = src.Agents
	}
	if len(src.Flows) > 0 {
		dst.Flows = src.Flows
	}
	if src.DefaultFlow != "" {
		dst.DefaultFlow = src.DefaultFlow
	}
}

// validateAgentsAndFlows checks agent/flow consistency.
// Returns nil if no agents are configured (single-agent mode).
// Invalid flows are filtered out and warnings are added to cfg.ValidationWarnings.
func (c *Config) validateAgentsAndFlows() error {
	if len(c.Agents) == 0 {
		return nil // Single-agent mode, nothing to validate
	}

	// Validate every agent has required fields
	for name, def := range c.Agents {
		// Set the name from the map key (not populated by YAML)
		def.Name = name
		c.Agents[name] = def

		if def.Model == "" {
			return fmt.Errorf("agent %q: model is required", name)
		}
		if def.Prompt == "" {
			return fmt.Errorf("agent %q: prompt is required", name)
		}
	}

	// Validate every flow references known agents
	// Filter out invalid flows instead of failing
	validFlows := make(map[string][]string)
	for flowName, steps := range c.Flows {
		valid := true
		var invalidAgent string
		for _, agentName := range steps {
			if _, ok := c.Agents[agentName]; !ok {
				valid = false
				invalidAgent = agentName
				break
			}
		}
		if valid {
			validFlows[flowName] = steps
		} else {
			c.ValidationWarnings = append(c.ValidationWarnings,
				fmt.Sprintf("flow %q skipped: references unknown agent %q", flowName, invalidAgent))
		}
	}
	c.Flows = validFlows

	// Validate default_flow references a known flow
	if c.DefaultFlow != "" {
		if _, ok := c.Flows[c.DefaultFlow]; !ok {
			c.ValidationWarnings = append(c.ValidationWarnings,
				fmt.Sprintf("default_flow %q is not a defined flow, clearing", c.DefaultFlow))
			c.DefaultFlow = ""
		}
	}

	return nil
}

// ResolveAgentConfig merges an AgentDefinition with the global Config values.
// Per-agent fields take precedence; zero/false/empty values fall back to globals.
func (c *Config) ResolveAgentConfig(name string) (flow.AgentConfig, error) {
	def, ok := c.Agents[name]
	if !ok {
		return flow.AgentConfig{}, fmt.Errorf("unknown agent: %q", name)
	}

	return flow.AgentConfig{
		Definition:              def,
		GlobalModel:             c.Model,
		GlobalEndpoint:          c.Endpoint,
		GlobalBearerToken:       c.BearerToken,
		GlobalMaxTokens:         c.MaxTokens,
		GlobalMaxContextTokens:    c.MaxContextTokens,
		GlobalCompactionThreshold: c.CompactionThreshold,
		GlobalKeepRecentTokens:    c.KeepRecentTokens,
	}, nil
}

// GetFlowDefinition returns a FlowDefinition for a named flow.
func (c *Config) GetFlowDefinition(name string) (flow.FlowDefinition, error) {
	steps, ok := c.Flows[name]
	if !ok {
		return flow.FlowDefinition{}, fmt.Errorf("unknown flow: %q", name)
	}
	return flow.FlowDefinition{
		Name:  name,
		Steps: steps,
	}, nil
}

// HasAgents returns true if multi-agent mode is configured.
func (c *Config) HasAgents() bool {
	return len(c.Agents) > 0
}

// HasFlows returns true if flows are configured.
func (c *Config) HasFlows() bool {
	return len(c.Flows) > 0
}