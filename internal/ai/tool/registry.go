package tool

import (
	"context"
	"sync"
)

// Tool describes an auxiliary capability that can be invoked by a provider.
type Tool interface {
	Name() string
	Summary() string
	InputSchema() map[string]any
	NewHandler() Handler
}

// Handler processes a single tool invocation.
type Handler interface {
	Invoke(ctx context.Context, input map[string]any) (Result, error)
}

// Invocation represents a request to execute a tool.
type Invocation struct {
	Name  string
	Input map[string]any
}

// Result contains the normalized output of a tool execution.
type Result struct {
	Output   map[string]any
	Metadata map[string]any
	Raw      any
}

// Descriptor contains metadata describing an available tool.
type Descriptor struct {
	Name        string
	Summary     string
	InputSchema map[string]any
}

// Registry manages runtime registration and lookup of tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry allocates an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register stores or replaces a tool implementation.
func (r *Registry) Register(tool Tool) {
	if tool == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name, if present.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// List exposes a snapshot of registered tools.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		out = append(out, tool)
	}
	return out
}

// Descriptors returns tool metadata ready to surface to LLM providers.
func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Descriptor, 0, len(r.tools))
	for _, tool := range r.tools {
		out = append(out, Descriptor{
			Name:        tool.Name(),
			Summary:     tool.Summary(),
			InputSchema: tool.InputSchema(),
		})
	}
	return out
}
