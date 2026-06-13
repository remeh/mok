package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/mok/internal/flow"
)

// TestParseYAMLWithAgentsAndFlows verifies that mok.yaml with agents and flows
// parses correctly into Config fields.
func TestParseYAMLWithAgentsAndFlows(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
model: "global-model"
endpoint: "http://global:8080/v1"
max_context_tokens: 131072
compaction_threshold: 0.8
keep_recent_tokens: 16384

agents:
  senior:
    model: "senior-model"
    prompt: "You are a senior architect."
  coder:
    model: "coder-model"
    prompt: "You are an expert coder."
    max_tokens: 8000
  reviewer:
    model: "reviewer-model"
    prompt: "You are a code reviewer."

flows:
  implementation: [senior, coder, reviewer, coder]
  review: [reviewer, coder]

default_flow: "implementation"
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Global fields
	if cfg.Model != "global-model" {
		t.Errorf("Model = %q, want global-model", cfg.Model)
	}
	if cfg.Endpoint != "http://global:8080/v1" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}

	// Agents
	if !cfg.HasAgents() {
		t.Fatal("HasAgents() = false, want true")
	}
	if len(cfg.Agents) != 3 {
		t.Fatalf("len(Agents) = %d, want 3", len(cfg.Agents))
	}

	senior := cfg.Agents["senior"]
	if senior.Name != "senior" {
		t.Errorf("senior.Name = %q, want senior", senior.Name)
	}
	if senior.Model != "senior-model" {
		t.Errorf("senior.Model = %q", senior.Model)
	}
	if senior.Prompt != "You are a senior architect." {
		t.Errorf("senior.Prompt = %q", senior.Prompt)
	}

	coder := cfg.Agents["coder"]
	if coder.MaxTokens != 8000 {
		t.Errorf("coder.MaxTokens = %d, want 8000", coder.MaxTokens)
	}

	// Flows
	if !cfg.HasFlows() {
		t.Fatal("HasFlows() = false, want true")
	}
	if cfg.DefaultFlow != "implementation" {
		t.Errorf("DefaultFlow = %q, want implementation", cfg.DefaultFlow)
	}

	implSteps := cfg.Flows["implementation"]
	if len(implSteps) != 4 {
		t.Errorf("len(implementation) = %d, want 4", len(implSteps))
	}

	reviewSteps := cfg.Flows["review"]
	if len(reviewSteps) != 2 {
		t.Errorf("len(review) = %d, want 2", len(reviewSteps))
	}
}

// TestParseYAMLWithoutAgents verifies backward compatibility:
// no agents → single-agent mode, LoadConfig succeeds.
func TestParseYAMLWithoutAgents(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
model: "simple-model"
endpoint: "http://simple:8080/v1"
max_tokens: 4096
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.HasAgents() {
		t.Error("HasAgents() = true, want false (no agents configured)")
	}
	if cfg.HasFlows() {
		t.Error("HasFlows() = true, want false (no flows configured)")
	}
	if cfg.Model != "simple-model" {
		t.Errorf("Model = %q, want simple-model", cfg.Model)
	}
}

// TestValidationMissingAgentReference checks that a flow referencing an
// unknown agent produces a clear error.
func TestValidationMissingAgentReference(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
agents:
  coder:
    model: "coder-model"
    prompt: "You are a coder."

flows:
  implementation: [senior, coder]
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := LoadConfig(nil)
	if err == nil {
		t.Fatal("expected error for unknown agent reference, got nil")
	}
}

// TestValidationAgentMissingModel verifies that an agent without a model field
// is rejected.
func TestValidationAgentMissingModel(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
agents:
  broken:
    prompt: "I have no model!"
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := LoadConfig(nil)
	if err == nil {
		t.Fatal("expected error for agent without model, got nil")
	}
}

// TestValidationAgentMissingPrompt verifies that an agent without a prompt field
// is rejected.
func TestValidationAgentMissingPrompt(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
agents:
  broken:
    model: "some-model"
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := LoadConfig(nil)
	if err == nil {
		t.Fatal("expected error for agent without prompt, got nil")
	}
}

// TestValidationBadDefaultFlow verifies that default_flow referencing
// an unknown flow is rejected.
func TestValidationBadDefaultFlow(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
agents:
  coder:
    model: "coder-model"
    prompt: "You are a coder."

default_flow: "nonexistent"
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := LoadConfig(nil)
	if err == nil {
		t.Fatal("expected error for bad default_flow, got nil")
	}
}

// TestResolveAgentConfig verifies that ResolveAgentConfig applies the
// per-agent vs global precedence correctly.
func TestResolveAgentConfig(t *testing.T) {
	cfg := &Config{
		Model:               "global-model",
		Endpoint:            "http://global:8080/v1",
		MaxTokens:           4096,
		MaxContextTokens:    131072,
		CompactionThreshold: 0.8,
		KeepRecentTokens:    16384,
		Agents: map[string]flow.AgentDefinition{
			"with-overrides": {
				Name:                "with-overrides",
				Model:               "override-model",
				Prompt:              "You override.",
				Endpoint:            "http://override:9999/v1",
				MaxTokens:           8000,
				MaxContextTokens:    65536,
				CompactionThreshold: 0.9,
				KeepRecentTokens:    8192,
			},
			"minimal": {
				Name:   "minimal",
				Model:  "minimal-model",
				Prompt: "You are minimal.",
				// All optionals left zero/empty → should inherit from global
			},
		},
	}

	// Agent with full overrides
	resolved, err := cfg.ResolveAgentConfig("with-overrides")
	if err != nil {
		t.Fatalf("ResolveAgentConfig: %v", err)
	}
	if resolved.ResolvedModel() != "override-model" {
		t.Errorf("ResolvedModel = %q", resolved.ResolvedModel())
	}
	if resolved.ResolvedEndpoint() != "http://override:9999/v1" {
		t.Errorf("ResolvedEndpoint = %q", resolved.ResolvedEndpoint())
	}
	if resolved.ResolvedMaxTokens() != 8000 {
		t.Errorf("ResolvedMaxTokens = %d", resolved.ResolvedMaxTokens())
	}
	if resolved.ResolvedMaxContextTokens() != 65536 {
		t.Errorf("ResolvedMaxContextTokens = %d", resolved.ResolvedMaxContextTokens())
	}
	if resolved.ResolvedCompactionThreshold() != 0.9 {
		t.Errorf("ResolvedCompactionThreshold = %f", resolved.ResolvedCompactionThreshold())
	}
	if resolved.ResolvedKeepRecentTokens() != 8192 {
		t.Errorf("ResolvedKeepRecentTokens = %d", resolved.ResolvedKeepRecentTokens())
	}

	// Agent with only required fields → inherits everything from global
	resolved2, err := cfg.ResolveAgentConfig("minimal")
	if err != nil {
		t.Fatalf("ResolveAgentConfig: %v", err)
	}
	if resolved2.ResolvedModel() != "minimal-model" {
		t.Errorf("ResolvedModel = %q", resolved2.ResolvedModel())
	}
	if resolved2.ResolvedEndpoint() != "http://global:8080/v1" {
		t.Errorf("ResolvedEndpoint = %q", resolved2.ResolvedEndpoint())
	}
	if resolved2.ResolvedMaxTokens() != 4096 {
		t.Errorf("ResolvedMaxTokens = %d", resolved2.ResolvedMaxTokens())
	}
	if resolved2.ResolvedMaxContextTokens() != 131072 {
		t.Errorf("ResolvedMaxContextTokens = %d", resolved2.ResolvedMaxContextTokens())
	}
	if resolved2.ResolvedCompactionThreshold() != 0.8 {
		t.Errorf("ResolvedCompactionThreshold = %f", resolved2.ResolvedCompactionThreshold())
	}
	if resolved2.ResolvedKeepRecentTokens() != 16384 {
		t.Errorf("ResolvedKeepRecentTokens = %d", resolved2.ResolvedKeepRecentTokens())
	}
}

// TestGetFlowDefinition verifies flow extraction.
func TestGetFlowDefinition(t *testing.T) {
	cfg := &Config{
		Flows: map[string][]string{
			"build": {"senior", "coder", "tester"},
		},
	}

	fd, err := cfg.GetFlowDefinition("build")
	if err != nil {
		t.Fatalf("GetFlowDefinition: %v", err)
	}
	if fd.Name != "build" {
		t.Errorf("Name = %q, want build", fd.Name)
	}
	if len(fd.Steps) != 3 {
		t.Errorf("len(Steps) = %d, want 3", len(fd.Steps))
	}

	_, err = cfg.GetFlowDefinition("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown flow, got nil")
	}
}

// TestHasAgentsHasFlowsEmpty verifies edge cases with empty maps.
func TestHasAgentsHasFlowsEmpty(t *testing.T) {
	cfg := &Config{}
	if cfg.HasAgents() {
		t.Error("HasAgents() = true for empty config")
	}
	if cfg.HasFlows() {
		t.Error("HasFlows() = true for empty config")
	}

	cfg2 := &Config{
		Agents: map[string]flow.AgentDefinition{},
		Flows:  map[string][]string{},
	}
	if cfg2.HasAgents() {
		t.Error("HasAgents() = true for zero-length map")
	}
	if cfg2.HasFlows() {
		t.Error("HasFlows() = true for zero-length map")
	}
}

// TestAgentDefinitionNamePreservation verifies that the Name field is
// set by validateAgentsAndFlows from the map key after loading.
func TestAgentDefinitionNamePreservation(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
agents:
  senior:
    model: "senior-model"
    prompt: "You are senior."
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	senior := cfg.Agents["senior"]
	if senior.Name != "senior" {
		t.Errorf("senior.Name = %q, want senior (set by validator)", senior.Name)
	}
}

// TestEnvOverridesAgents verifies that environment variables can override
// global-level fields, and agents inherit from the resolved global config.
func TestEnvOverridesAgents(t *testing.T) {
	os.Setenv("MOK_MODEL", "env-global-model")
	t.Cleanup(func() { os.Unsetenv("MOK_MODEL") })

	tmpDir := t.TempDir()

	yamlContent := `
model: "file-global-model"
endpoint: "http://file:8080/v1"

agents:
  coder:
    model: "coder-model"
    prompt: "You are a coder."
`
	yamlPath := filepath.Join(tmpDir, "mok.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Global model comes from env (overrides file-level default)
	if cfg.Model != "env-global-model" {
		t.Errorf("Model = %q, want env-global-model", cfg.Model)
	}

	resolved, err := cfg.ResolveAgentConfig("coder")
	if err != nil {
		t.Fatalf("ResolveAgentConfig: %v", err)
	}

	// Agent has its own model → that takes precedence over global
	if resolved.ResolvedModel() != "coder-model" {
		t.Errorf("ResolvedModel = %q, want coder-model (agent override)", resolved.ResolvedModel())
	}

	// Agent has no endpoint → falls back to global (which comes from file)
	if resolved.ResolvedEndpoint() != "http://file:8080/v1" {
		t.Errorf("ResolvedEndpoint = %q, want http://file:8080/v1", resolved.ResolvedEndpoint())
	}
}
