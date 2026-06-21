package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateAgentsAndFlows_GracefulDegradation tests that invalid flows
// are filtered out gracefully with warnings instead of failing config load.
func TestValidateAgentsAndFlows_GracefulDegradation(t *testing.T) {
	// Create a temp config file with invalid flow references
	cfgContent := `
model: test-model
endpoint: http://localhost:8080/v1
max_context_tokens: 131072

agents:
  coder:
    model: qwen3.6-27b-coder
    prompt: "You are an expert developer."
  reviewer:
    model: qwen3.5-122b-coder
    prompt: "You are an expert code reviewer."

flows:
  valid-flow: [coder, reviewer]
  invalid-flow: [coder, unknown_agent, reviewer]
  another-invalid: [nonexistent]

default_flow: "valid-flow"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(configPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Change to temp dir so loadFromFile finds our config
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// Load config - should succeed with warnings
	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig should not fail: %v", err)
	}

	// Check that valid flows are preserved
	if _, ok := cfg.Flows["valid-flow"]; !ok {
		t.Error("valid-flow should be preserved")
	}

	// Check that invalid flows are removed
	if _, ok := cfg.Flows["invalid-flow"]; ok {
		t.Error("invalid-flow should be removed")
	}
	if _, ok := cfg.Flows["another-invalid"]; ok {
		t.Error("another-invalid should be removed")
	}

	// Check that warnings were generated
	if len(cfg.ValidationWarnings) == 0 {
		t.Error("Expected validation warnings")
	}

	// Check warning messages contain expected info
	foundInvalidFlowWarning := false
	for _, w := range cfg.ValidationWarnings {
		t.Logf("Warning: %s", w)
		if strings.Contains(w, "flow") && strings.Contains(w, "skipped") {
			foundInvalidFlowWarning = true
		}
		if strings.Contains(w, "unknown agent") {
			foundInvalidFlowWarning = true
		}
	}

	if !foundInvalidFlowWarning {
		t.Error("Expected warning about invalid flow with unknown agent")
	}

	// Default flow should still be valid
	if cfg.DefaultFlow != "valid-flow" {
		t.Errorf("Expected default_flow to be 'valid-flow', got %q", cfg.DefaultFlow)
	}
}

// TestValidateAgentsAndFlows_InvalidDefaultFlow tests that invalid default_flow
// is cleared with a warning.
func TestValidateAgentsAndFlows_InvalidDefaultFlow(t *testing.T) {
	cfgContent := `
model: test-model
endpoint: http://localhost:8080/v1

agents:
  coder:
    model: qwen3.6-27b-coder
    prompt: "You are an expert developer."

flows:
  valid-flow: [coder]

default_flow: "nonexistent-flow"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(configPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig should not fail: %v", err)
	}

	// Default flow should be cleared
	if cfg.DefaultFlow != "" {
		t.Errorf("Expected default_flow to be cleared, got %q", cfg.DefaultFlow)
	}

	// Should have a warning
	foundWarning := false
	for _, w := range cfg.ValidationWarnings {
		t.Logf("Warning: %s", w)
		if strings.Contains(w, "default_flow") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("Expected warning about invalid default_flow")
	}
}
