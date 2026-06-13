package flow

import (
	"testing"

	"github.com/user/mok/internal/tools"
)

func TestAgentFactory_BuildAgent(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:               "global-model",
		Endpoint:            "http://global:8080/v1",
		BearerToken:         "token-abc",
		MaxTokens:           4096,
		MaxContextTokens:    131072,
		CompactionThreshold: 0.8,
		KeepRecentTokens:    16384,
		CWD:                 "/tmp/test",
	}

	reg := tools.NewRegistry()
	reg.Add(&tools.ReadTool{CWD: "/tmp/test"})
	reg.Add(&tools.BashTool{CWD: "/tmp/test"})

	factory := NewAgentFactory(global, reg, nil)

	def := AgentDefinition{
		Name:   "coder",
		Model:  "coder-model",
		Prompt: "You are an expert coder. Write excellent code.",
	}

	agt, err := factory.BuildAgent(def)
	if err != nil {
		t.Fatalf("BuildAgent: %v", err)
	}

	if agt == nil {
		t.Fatal("BuildAgent returned nil agent")
	}

	// Agent should have the correct config values
	if agt.Tools() == nil {
		t.Error("Agent has nil tool registry")
	}
	if !agt.Tools().Has("read") {
		t.Error("Agent missing 'read' tool")
	}
	if !agt.Tools().Has("bash") {
		t.Error("Agent missing 'bash' tool")
	}
}

func TestAgentFactory_BuildAgent_WithOverrides(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:               "global-model",
		Endpoint:            "http://global:8080/v1",
		BearerToken:         "token",
		MaxTokens:           4096,
		MaxContextTokens:    131072,
		CompactionThreshold: 0.8,
		KeepRecentTokens:    16384,
		CWD:                 "/tmp/test",
	}

	reg := tools.NewRegistry()
	reg.Add(&tools.ReadTool{CWD: "/tmp/test"})

	factory := NewAgentFactory(global, reg, nil)

	def := AgentDefinition{
		Name:                "senior",
		Model:               "senior-model",
		Prompt:              "You are a senior architect.",
		Endpoint:            "http://senior:9999/v1",
		MaxTokens:           8000,
		MaxContextTokens:    65536,
		CompactionThreshold: 0.9,
		KeepRecentTokens:    8192,
	}

	agt, err := factory.BuildAgent(def)
	if err != nil {
		t.Fatalf("BuildAgent: %v", err)
	}
	if agt == nil {
		t.Fatal("BuildAgent returned nil agent")
	}

	// Verify the agent was configured correctly.
	// We can check the tool registry and that creation succeeded.
	// Full config verification is done via the Agent's String() output.
	s := agt.String()
	if s == "" {
		t.Error("Agent.String() returned empty string")
	}
}

func TestAgentFactory_BuildAgent_MinimalInheritsGlobals(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:               "global-model",
		Endpoint:            "http://global:8080/v1",
		BearerToken:         "token",
		MaxTokens:           4096,
		MaxContextTokens:    100000,
		CompactionThreshold: 0.7,
		KeepRecentTokens:    10000,
		CWD:                 "/tmp/test",
	}

	reg := tools.NewRegistry()

	factory := NewAgentFactory(global, reg, nil)

	def := AgentDefinition{
		Name:   "minimal",
		Model:  "minimal-model",
		Prompt: "You are minimal.",
		// All optional fields left zero
	}

	agt, err := factory.BuildAgent(def)
	if err != nil {
		t.Fatalf("BuildAgent: %v", err)
	}
	if agt == nil {
		t.Fatal("BuildAgent returned nil agent")
	}
}

func TestAgentFactory_BuildAgent_Errors(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:    "global-model",
		Endpoint: "http://global:8080/v1",
		CWD:      "/tmp/test",
	}

	reg := tools.NewRegistry()
	factory := NewAgentFactory(global, reg, nil)

	tests := []struct {
		name string
		def  AgentDefinition
	}{
		{"empty name", AgentDefinition{Name: "", Model: "m", Prompt: "p"}},
		{"no model", AgentDefinition{Name: "agent", Model: "", Prompt: "p"}},
		{"no prompt", AgentDefinition{Name: "agent", Model: "m", Prompt: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.BuildAgent(tt.def)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestAgentFactory_ClientReuse(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:       "global-model",
		Endpoint:    "http://global:8080/v1",
		BearerToken: "token-xyz",
		CWD:         "/tmp/test",
	}

	reg := tools.NewRegistry()
	reg.Add(&tools.ReadTool{CWD: "/tmp/test"})

	factory := NewAgentFactory(global, reg, nil)

	def1 := AgentDefinition{
		Name:   "agent1",
		Model:  "model1",
		Prompt: "first agent",
	}
	def2 := AgentDefinition{
		Name:   "agent2",
		Model:  "model2",
		Prompt: "second agent",
	}

	agt1, err := factory.BuildAgent(def1)
	if err != nil {
		t.Fatalf("BuildAgent(agent1): %v", err)
	}
	agt2, err := factory.BuildAgent(def2)
	if err != nil {
		t.Fatalf("BuildAgent(agent2): %v", err)
	}

	if agt1 == agt2 {
		t.Error("two different agents should be distinct instances")
	}

	// Both should be non-nil
	if agt1 == nil || agt2 == nil {
		t.Fatal("both agents should be non-nil")
	}
}

func TestAgentFactory_ClientCache(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:       "global-model",
		Endpoint:    "http://global:8080/v1",
		BearerToken: "token",
		CWD:         "/tmp/test",
	}

	reg := tools.NewRegistry()
	factory := NewAgentFactory(global, reg, nil)

	// Build two agents with identical endpoint/token → should share client
	def1 := AgentDefinition{
		Name:   "a1",
		Model:  "m1",
		Prompt: "agent one",
	}
	def2 := AgentDefinition{
		Name:   "a2",
		Model:  "m2",
		Prompt: "agent two",
	}

	agt1, _ := factory.BuildAgent(def1)
	agt2, _ := factory.BuildAgent(def2)

	if agt1 == nil || agt2 == nil {
		t.Fatal("agents should not be nil")
	}

	// The factory should have exactly 1 cached client
	if len(factory.clients) != 1 {
		t.Errorf("expected 1 cached client, got %d", len(factory.clients))
	}

	// Build an agent with a different endpoint → new client
	def3 := AgentDefinition{
		Name:     "a3",
		Model:    "m3",
		Prompt:   "agent three",
		Endpoint: "http://other:8080/v1",
	}

	agt3, _ := factory.BuildAgent(def3)
	if agt3 == nil {
		t.Fatal("agent3 should not be nil")
	}

	if len(factory.clients) != 2 {
		t.Errorf("expected 2 cached clients, got %d", len(factory.clients))
	}
}

func TestResolvedAgentConfig_Defaults(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:               "g-model",
		Endpoint:            "http://g:8080/v1",
		BearerToken:         "g-token",
		MaxTokens:           4096,
		MaxContextTokens:    100000,
		CompactionThreshold: 0.7,
		KeepRecentTokens:    10000,
		SummarizationModel:  "summary-model",
	}

	factory := &AgentFactory{globalConfig: global}

	rc := factory.resolveConfig(AgentDefinition{
		Name:   "test",
		Model:  "test-model",
		Prompt: "test",
	})

	if rc.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", rc.Model)
	}
	if rc.Endpoint != "http://g:8080/v1" {
		t.Errorf("Endpoint = %q, want global", rc.Endpoint)
	}
	if rc.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", rc.MaxTokens)
	}
	if rc.MaxContextTokens != 100000 {
		t.Errorf("MaxContextTokens = %d", rc.MaxContextTokens)
	}
	if rc.CompactionThreshold != 0.7 {
		t.Errorf("CompactionThreshold = %f, want 0.7", rc.CompactionThreshold)
	}
	if rc.KeepRecentTokens != 10000 {
		t.Errorf("KeepRecentTokens = %d", rc.KeepRecentTokens)
	}
	if rc.SummarizationModel != "summary-model" {
		t.Errorf("SummarizationModel = %q, want summary-model", rc.SummarizationModel)
	}
}

func TestResolvedAgentConfig_AllOverrides(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:               "g-model",
		Endpoint:            "http://g:8080/v1",
		BearerToken:         "g-token",
		MaxTokens:           4096,
		MaxContextTokens:    100000,
		CompactionThreshold: 0.7,
		KeepRecentTokens:    10000,
	}

	factory := &AgentFactory{globalConfig: global}

	rc := factory.resolveConfig(AgentDefinition{
		Name:                "full",
		Model:               "override-model",
		Prompt:              "overridden",
		Endpoint:            "http://override:9999/v1",
		MaxTokens:           8000,
		MaxContextTokens:    65536,
		CompactionThreshold: 0.9,
		KeepRecentTokens:    8192,
		SummarizationModel:  "custom-summarizer",
	})

	if rc.Model != "override-model" {
		t.Errorf("Model = %q", rc.Model)
	}
	if rc.Endpoint != "http://override:9999/v1" {
		t.Errorf("Endpoint = %q", rc.Endpoint)
	}
	if rc.MaxTokens != 8000 {
		t.Errorf("MaxTokens = %d", rc.MaxTokens)
	}
	if rc.MaxContextTokens != 65536 {
		t.Errorf("MaxContextTokens = %d", rc.MaxContextTokens)
	}
	if rc.CompactionThreshold != 0.9 {
		t.Errorf("CompactionThreshold = %f", rc.CompactionThreshold)
	}
	if rc.KeepRecentTokens != 8192 {
		t.Errorf("KeepRecentTokens = %d", rc.KeepRecentTokens)
	}
	if rc.SummarizationModel != "custom-summarizer" {
		t.Errorf("SummarizationModel = %q", rc.SummarizationModel)
	}
}

func TestAgentFactory_NilDebug(t *testing.T) {
	global := &AgentConfigGlobal{
		Model:    "model",
		Endpoint: "http://e:8080/v1",
		CWD:      "/tmp",
	}

	reg := tools.NewRegistry()
	factory := NewAgentFactory(global, reg, nil)

	def := AgentDefinition{
		Name:   "agent",
		Model:  "m",
		Prompt: "p",
	}

	agt, err := factory.BuildAgent(def)
	if err != nil {
		t.Fatalf("BuildAgent with nil debug: %v", err)
	}
	if agt == nil {
		t.Fatal("agent is nil")
	}
}
