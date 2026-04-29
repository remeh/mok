package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "qwen3-8b" {
		t.Errorf("default model = %q, want %q", cfg.Model, "qwen3-8b")
	}
	if cfg.Endpoint != "http://localhost:8080/v1" {
		t.Errorf("default endpoint = %q", cfg.Endpoint)
	}
	if cfg.MaxContextTokens != 131072 {
		t.Errorf("default max_context_tokens = %d", cfg.MaxContextTokens)
	}
	if cfg.CompactionThreshold != 0.8 {
		t.Errorf("default compaction_threshold = %f", cfg.CompactionThreshold)
	}
	if cfg.KeepRecentTokens != 16384 {
		t.Errorf("default keep_recent_tokens = %d", cfg.KeepRecentTokens)
	}
	if cfg.Temperature != 0.0 {
		t.Errorf("default temperature = %f", cfg.Temperature)
	}
	if cfg.MaxTokens != 0 {
		t.Errorf("default max_tokens = %d", cfg.MaxTokens)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `model: "llama-3.1-8b"
endpoint: "http://192.168.1.1:9000/v1"
api_key: "test-key-123"
max_context_tokens: 65536
compaction_threshold: 0.9
keep_recent_tokens: 8192
temperature: 0.5
max_tokens: 4096
`
	yamlPath := filepath.Join(tmpDir, "mmok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir so loadFromFile finds it
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Model != "llama-3.1-8b" {
		t.Errorf("model = %q, want %q", cfg.Model, "llama-3.1-8b")
	}
	if cfg.Endpoint != "http://192.168.1.1:9000/v1" {
		t.Errorf("endpoint = %q", cfg.Endpoint)
	}
	if cfg.APIKey != "test-key-123" {
		t.Errorf("api_key = %q", cfg.APIKey)
	}
	if cfg.MaxContextTokens != 65536 {
		t.Errorf("max_context_tokens = %d", cfg.MaxContextTokens)
	}
	if cfg.CompactionThreshold != 0.9 {
		t.Errorf("compaction_threshold = %f", cfg.CompactionThreshold)
	}
	if cfg.KeepRecentTokens != 8192 {
		t.Errorf("keep_recent_tokens = %d", cfg.KeepRecentTokens)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("max_tokens = %d", cfg.MaxTokens)
	}
}

func TestLoadConfigEnvVars(t *testing.T) {
	// Set env vars
	os.Setenv("MMOK_MODEL", "env-model")
	os.Setenv("MMOK_ENDPOINT", "http://env-host:9999/v1")
	os.Setenv("MMOK_API_KEY", "env-key")
	os.Setenv("MMOK_MAX_CONTEXT_TOKENS", "32768")
	os.Setenv("MMOK_KEEP_RECENT_TOKENS", "4096")
	os.Setenv("MMOK_TEMPERATURE", "0.7")
	os.Setenv("MMOK_MAX_TOKENS", "2048")
	t.Cleanup(func() {
		os.Unsetenv("MMOK_MODEL")
		os.Unsetenv("MMOK_ENDPOINT")
		os.Unsetenv("MMOK_API_KEY")
		os.Unsetenv("MMOK_MAX_CONTEXT_TOKENS")
		os.Unsetenv("MMOK_KEEP_RECENT_TOKENS")
		os.Unsetenv("MMOK_TEMPERATURE")
		os.Unsetenv("MMOK_MAX_TOKENS")
	})

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Model != "env-model" {
		t.Errorf("model = %q, want %q", cfg.Model, "env-model")
	}
	if cfg.Endpoint != "http://env-host:9999/v1" {
		t.Errorf("endpoint = %q", cfg.Endpoint)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("api_key = %q", cfg.APIKey)
	}
	if cfg.MaxContextTokens != 32768 {
		t.Errorf("max_context_tokens = %d", cfg.MaxContextTokens)
	}
	if cfg.KeepRecentTokens != 4096 {
		t.Errorf("keep_recent_tokens = %d", cfg.KeepRecentTokens)
	}
	if cfg.MaxTokens != 2048 {
		t.Errorf("max_tokens = %d", cfg.MaxTokens)
	}
}

func TestLoadConfigFlagsOverride(t *testing.T) {
	os.Setenv("MMOK_MODEL", "env-model")
	t.Cleanup(func() { os.Unsetenv("MMOK_MODEL") })

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	flags := map[string]string{
		"model":    "flag-model",
		"endpoint": "http://flag-host:7777/v1",
	}

	cfg, err := LoadConfig(flags)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Model != "flag-model" {
		t.Errorf("model = %q, want flag-model (flags should override env)", cfg.Model)
	}
	if cfg.Endpoint != "http://flag-host:7777/v1" {
		t.Errorf("endpoint = %q", cfg.Endpoint)
	}
}

func TestLoadConfigPrecedence(t *testing.T) {
	// Write a file with model="file-model"
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "mmok.yaml")
	os.WriteFile(yamlPath, []byte("model: \"file-model\"\nendpoint: \"http://file:1111/v1\"\n"), 0644)

	// Set env with model="env-model"
	os.Setenv("MMOK_MODEL", "env-model")
	os.Setenv("MMOK_ENDPOINT", "http://env:2222/v1")
	t.Cleanup(func() {
		os.Unsetenv("MMOK_MODEL")
		os.Unsetenv("MMOK_ENDPOINT")
	})

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Flags override everything
	flags := map[string]string{
		"model": "flag-model",
	}

	cfg, err := LoadConfig(flags)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Model: flags > env > file
	if cfg.Model != "flag-model" {
		t.Errorf("model = %q, want flag-model", cfg.Model)
	}
	// Endpoint: env > file (no flag for endpoint)
	if cfg.Endpoint != "http://env:2222/v1" {
		t.Errorf("endpoint = %q, want http://env:2222/v1 (env should override file)", cfg.Endpoint)
	}
}

func TestLoadConfigNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig should not fail when no config file exists: %v", err)
	}

	// Should fall back to defaults
	if cfg.Model != "qwen3-8b" {
		t.Errorf("model = %q, want default", cfg.Model)
	}
}

func TestMergeConfig(t *testing.T) {
	dst := DefaultConfig()
	src := &Config{
		Model:            "override",
		MaxContextTokens: 64000,
	}

	mergeConfig(dst, src)

	if dst.Model != "override" {
		t.Errorf("model = %q, want override", dst.Model)
	}
	if dst.MaxContextTokens != 64000 {
		t.Errorf("max_context_tokens = %d, want 64000", dst.MaxContextTokens)
	}
	// Unchanged fields should keep default values
	if dst.Endpoint != "http://localhost:8080/v1" {
		t.Errorf("endpoint = %q, want default", dst.Endpoint)
	}
}

func TestApplyFlags(t *testing.T) {
	cfg := DefaultConfig()
	flags := map[string]string{
		"model":              "flag-model",
		"max-context-tokens": "128000",
		"temperature":        "0.3",
		"max-tokens":         "8192",
	}

	applyFlags(cfg, flags)

	if cfg.Model != "flag-model" {
		t.Errorf("model = %q, want flag-model", cfg.Model)
	}
	if cfg.MaxContextTokens != 128000 {
		t.Errorf("max_context_tokens = %d, want 128000", cfg.MaxContextTokens)
	}
	if cfg.Temperature != 0.3 {
		t.Errorf("temperature = %f, want 0.3", cfg.Temperature)
	}
	if cfg.MaxTokens != 8192 {
		t.Errorf("max_tokens = %d, want 8192", cfg.MaxTokens)
	}
}

func TestApplyFlagsEmptyStrings(t *testing.T) {
	cfg := DefaultConfig()
	flags := map[string]string{
		"model": "",
	}

	applyFlags(cfg, flags)

	// Empty strings should not override
	if cfg.Model != "qwen3-8b" {
		t.Errorf("model = %q, want default (empty flag should not override)", cfg.Model)
	}
}

func TestLoadFromEnvInvalidValues(t *testing.T) {
	os.Setenv("MMOK_MAX_CONTEXT_TOKENS", "not-a-number")
	os.Setenv("MMOK_TEMPERATURE", "not-a-number")
	os.Setenv("MMOK_MAX_TOKENS", "not-a-number")
	t.Cleanup(func() {
		os.Unsetenv("MMOK_MAX_CONTEXT_TOKENS")
		os.Unsetenv("MMOK_TEMPERATURE")
		os.Unsetenv("MMOK_MAX_TOKENS")
	})

	cfg := DefaultConfig()
	if err := loadFromEnv(cfg); err != nil {
		t.Fatalf("loadFromEnv should not fail on invalid values: %v", err)
	}

	// Should keep defaults when values are invalid
	if cfg.MaxContextTokens != 131072 {
		t.Errorf("max_context_tokens = %d, want default", cfg.MaxContextTokens)
	}
}