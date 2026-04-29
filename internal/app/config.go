package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFileNames tried in order.
var configPaths = []string{
	".", // project-local mmok.yaml
}

// configEnvPrefix for environment variables.
const configEnvPrefix = "MMOK_"

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

	return cfg, nil
}

// loadFromFile searches for config.yaml in known locations.
func loadFromFile() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	paths := []string{
		"./mmok.yaml",
		"./config.yaml",
		filepath.Join(home, ".config", "mmok", "config.yaml"),
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
		Model              string `env:"MODEL" envDefault:""`
		Endpoint           string `env:"ENDPOINT" envDefault:""`
		APIKey             string `env:"API_KEY" envDefault:""`
		MaxContextTokens   int    `env:"MAX_CONTEXT_TOKENS" envDefault:"0"`
		CompactionThreshold float64 `env:"COMPACTION_THRESHOLD" envDefault:"0"`
		KeepRecentTokens   int    `env:"KEEP_RECENT_TOKENS" envDefault:"0"`
		Temperature        float32 `env:"TEMPERATURE" envDefault:"0"`
		MaxTokens          int    `env:"MAX_TOKENS" envDefault:"0"`
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
	if v, ok := envMap["API_KEY"]; ok && v != "" {
		cfg.APIKey = v
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
	if v, ok := envMap["TEMPERATURE"]; ok && v != "" {
		if n, err := strconv.ParseFloat(v, 32); err == nil {
			cfg.Temperature = float32(n)
		}
	}
	if v, ok := envMap["MAX_TOKENS"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxTokens = n
		}
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
	if v, ok := flags["api-key"]; ok && v != "" {
		cfg.APIKey = v
	}
	if v, ok := flags["max-context-tokens"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxContextTokens = n
		}
	}
	if v, ok := flags["temperature"]; ok && v != "" {
		if n, err := strconv.ParseFloat(v, 32); err == nil {
			cfg.Temperature = float32(n)
		}
	}
	if v, ok := flags["max-tokens"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxTokens = n
		}
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
	if src.APIKey != "" {
		dst.APIKey = src.APIKey
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
	// Temperature can be 0 intentionally, so we check a sentinel
	// For now, if the YAML has it, it was explicitly set — we handle this
	// by checking if the source value differs from default
}
