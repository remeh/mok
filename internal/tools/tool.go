package tools

import (
	"encoding/json"
	"sort"

	"github.com/user/mmok/internal/llm"
)

// ToolDefinition is the flat domain type for a tool's metadata.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Snippet     string         `json:"-"`          // One-line summary for the system prompt (not sent to API)
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// Tool is the executable interface.
type Tool interface {
	Definition() ToolDefinition
	Execute(args json.RawMessage) (string, error)
}

// Registry holds available tools and converts to wire format.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Add registers a tool. Panics if a tool with the same name already exists.
func (r *Registry) Add(tool Tool) {
	name := tool.Definition().Name
	if _, exists := r.tools[name]; exists {
		panic("tool already registered: " + name)
	}
	r.tools[name] = tool
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

// All returns all registered tools sorted by name. Sorting keeps the order
// deterministic across calls and processes so the system prompt and tool
// list stay stable for prompt-cache prefix reuse.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Definition().Name < result[j].Definition().Name
	})
	return result
}

// Has returns true if a tool with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// ToSpecs converts all registered tools to the API wire format. Specs are
// sorted by name so the wire-format tools array is byte-identical across
// requests, which is required for prompt-cache prefix matching.
func (r *Registry) ToSpecs() []llm.ToolSpec {
	all := r.All()
	specs := make([]llm.ToolSpec, 0, len(all))
	for _, t := range all {
		def := t.Definition()
		specs = append(specs, llm.ToolSpec{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return specs
}
