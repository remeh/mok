package flow

import "testing"

func TestAgentConfig_ResolutionPrecedence(t *testing.T) {
	ac := AgentConfig{
		Definition: AgentDefinition{
			Name:   "coder",
			Model:  "code-model-local",
			Prompt: "You are a coder.",
		},
		GlobalModel:             "default-model",
		GlobalEndpoint:          "http://global:8080/v1",
		GlobalMaxTokens:         4096,
		GlobalMaxContextTokens:    131072,
		GlobalCompactionThreshold: 0.8,
		GlobalKeepRecentTokens:    16384,
	}

	// Agent-specific model wins over global
	if ac.ResolvedModel() != "code-model-local" {
		t.Errorf("ResolvedModel() = %q, want code-model-local", ac.ResolvedModel())
	}

	// Endpoint not set on agent → falls back to global
	if ac.ResolvedEndpoint() != "http://global:8080/v1" {
		t.Errorf("ResolvedEndpoint() = %q, want http://global:8080/v1", ac.ResolvedEndpoint())
	}

	// MaxTokens not set on agent → falls back to global
	if ac.ResolvedMaxTokens() != 4096 {
		t.Errorf("ResolvedMaxTokens() = %d, want 4096", ac.ResolvedMaxTokens())
	}

	// MaxContextTokens not set on agent → falls back to global
	if ac.ResolvedMaxContextTokens() != 131072 {
		t.Errorf("ResolvedMaxContextTokens() = %d, want 131072", ac.ResolvedMaxContextTokens())
	}

	// CompactionThreshold not set → falls back to global
	if ac.ResolvedCompactionThreshold() != 0.8 {
		t.Errorf("ResolvedCompactionThreshold() = %f, want 0.8", ac.ResolvedCompactionThreshold())
	}

	// KeepRecentTokens not set → falls back to global
	if ac.ResolvedKeepRecentTokens() != 16384 {
		t.Errorf("ResolvedKeepRecentTokens() = %d, want 16384", ac.ResolvedKeepRecentTokens())
	}
}

func TestAgentConfig_AgentOverridesGlobal(t *testing.T) {
	ac := AgentConfig{
		Definition: AgentDefinition{
			Name:                "senior",
			Model:               "senior-model",
			Endpoint:            "http://senior:9999/v1",
			MaxTokens:           8192,
			MaxContextTokens:    65536,
			CompactionThreshold: 0.9,
			KeepRecentTokens:    8192,
		},
		GlobalModel:             "default-model",
		GlobalEndpoint:          "http://default:8080/v1",
		GlobalMaxTokens:         4096,
		GlobalMaxContextTokens:    131072,
		GlobalCompactionThreshold: 0.8,
		GlobalKeepRecentTokens:    16384,
	}

	if ac.ResolvedModel() != "senior-model" {
		t.Errorf("ResolvedModel() = %q, want senior-model", ac.ResolvedModel())
	}
	if ac.ResolvedEndpoint() != "http://senior:9999/v1" {
		t.Errorf("ResolvedEndpoint() = %q", ac.ResolvedEndpoint())
	}
	if ac.ResolvedMaxTokens() != 8192 {
		t.Errorf("ResolvedMaxTokens() = %d, want 8192", ac.ResolvedMaxTokens())
	}
	if ac.ResolvedMaxContextTokens() != 65536 {
		t.Errorf("ResolvedMaxContextTokens() = %d, want 65536", ac.ResolvedMaxContextTokens())
	}
	if ac.ResolvedCompactionThreshold() != 0.9 {
		t.Errorf("ResolvedCompactionThreshold() = %f, want 0.9", ac.ResolvedCompactionThreshold())
	}
	if ac.ResolvedKeepRecentTokens() != 8192 {
		t.Errorf("ResolvedKeepRecentTokens() = %d, want 8192", ac.ResolvedKeepRecentTokens())
	}
}

func TestAgentConfig_ZeroFallback(t *testing.T) {
	// Zero values on the agent should fall back to global
	ac := AgentConfig{
		Definition: AgentDefinition{
			Name:   "minimal",
			Model:  "minimal-model",
			Prompt: "You are minimal.",
			// All optional fields are zero/empty
		},
		GlobalModel:             "global-model",
		GlobalEndpoint:          "http://global:8080/v1",
		GlobalMaxTokens:         2048,
		GlobalMaxContextTokens:    100000,
		GlobalCompactionThreshold: 0.7,
		GlobalKeepRecentTokens:    10000,
	}

	if ac.ResolvedModel() != "minimal-model" {
		t.Errorf("ResolvedModel() = %q, want minimal-model", ac.ResolvedModel())
	}
	if ac.ResolvedEndpoint() != "http://global:8080/v1" {
		t.Errorf("ResolvedEndpoint() = %q, want http://global:8080/v1", ac.ResolvedEndpoint())
	}
	if ac.ResolvedMaxTokens() != 2048 {
		t.Errorf("ResolvedMaxTokens() = %d, want 2048", ac.ResolvedMaxTokens())
	}
	if ac.ResolvedMaxContextTokens() != 100000 {
		t.Errorf("ResolvedMaxContextTokens() = %d, want 100000", ac.ResolvedMaxContextTokens())
	}
	if ac.ResolvedCompactionThreshold() != 0.7 {
		t.Errorf("ResolvedCompactionThreshold() = %f, want 0.7", ac.ResolvedCompactionThreshold())
	}
	if ac.ResolvedKeepRecentTokens() != 10000 {
		t.Errorf("ResolvedKeepRecentTokens() = %d, want 10000", ac.ResolvedKeepRecentTokens())
	}
}

func TestAgentDefinition_FieldMapping(t *testing.T) {
	def := AgentDefinition{
		Name:                "test-agent",
		Model:               "test-model",
		Prompt:              "You are a test agent.",
		Endpoint:            "http://custom:8080/v1",
		MaxTokens:           500,
		MaxContextTokens:    10000,
		CompactionThreshold: 0.5,
		KeepRecentTokens:    2000,
		SummarizationModel:  "summary-model",
	}

	if def.Name != "test-agent" {
		t.Errorf("Name = %q", def.Name)
	}
	if def.Model != "test-model" {
		t.Errorf("Model = %q", def.Model)
	}
	if def.Prompt != "You are a test agent." {
		t.Errorf("Prompt = %q", def.Prompt)
	}
	if def.Endpoint != "http://custom:8080/v1" {
		t.Errorf("Endpoint = %q", def.Endpoint)
	}
	if def.MaxTokens != 500 {
		t.Errorf("MaxTokens = %d", def.MaxTokens)
	}
	if def.MaxContextTokens != 10000 {
		t.Errorf("MaxContextTokens = %d", def.MaxContextTokens)
	}
	if def.CompactionThreshold != 0.5 {
		t.Errorf("CompactionThreshold = %f", def.CompactionThreshold)
	}
	if def.KeepRecentTokens != 2000 {
		t.Errorf("KeepRecentTokens = %d", def.KeepRecentTokens)
	}
	if def.SummarizationModel != "summary-model" {
		t.Errorf("SummarizationModel = %q", def.SummarizationModel)
	}
}

func TestFlowDefinition_Fields(t *testing.T) {
	fd := FlowDefinition{
		Name:  "my-flow",
		Steps: []string{"senior", "coder", "reviewer"},
	}

	if fd.Name != "my-flow" {
		t.Errorf("Name = %q", fd.Name)
	}
	if len(fd.Steps) != 3 {
		t.Errorf("len(Steps) = %d, want 3", len(fd.Steps))
	}
	if fd.Steps[0] != "senior" || fd.Steps[1] != "coder" || fd.Steps[2] != "reviewer" {
		t.Errorf("Steps = %v, want [senior coder reviewer]", fd.Steps)
	}
}
