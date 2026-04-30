package tools

import (
	"encoding/json"

	"github.com/user/mmok/internal/llm"
)

// ToolDefinition is the flat domain type for a tool's metadata.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
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

// All returns all registered tools in insertion order.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Has returns true if a tool with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// ToSpecs converts all registered tools to the API wire format.
func (r *Registry) ToSpecs() []llm.ToolSpec {
	specs := make([]llm.ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
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
