// Package plugin defines the platform-wide plugin/tool SDK.
// Plugins are self-contained capability units that can be loaded by any service.
package plugin

import (
	"context"
	"encoding/json"
	"time"
)

// ─── Plugin manifest ───────────────────────────────────────────────────────────

// Manifest describes a plugin's identity and capabilities.
type Manifest struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Author      string       `json:"author"`
	Capabilities []Capability `json:"capabilities"`
	Schema      json.RawMessage `json:"schema,omitempty"` // JSON Schema for input
}

type Capability string

const (
	CapabilityTool      Capability = "tool"       // callable by AI agents
	CapabilitySource    Capability = "source"      // data ingestion source
	CapabilityAction    Capability = "action"      // side-effect action
	CapabilityTransform Capability = "transform"   // data transformation
	CapabilityTrigger   Capability = "trigger"     // workflow trigger
)

// ─── Execution ────────────────────────────────────────────────────────────────

// Input carries the parameters passed to a plugin.
type Input struct {
	Params  json.RawMessage `json:"params"`
	Context map[string]any  `json:"context,omitempty"` // session/user context
}

// Output is returned by a plugin execution.
type Output struct {
	Data    json.RawMessage `json:"data,omitempty"`
	Text    string          `json:"text,omitempty"` // human-readable summary
	Error   string          `json:"error,omitempty"`
	Latency time.Duration   `json:"latency_ms"`
}

// ─── Interfaces ───────────────────────────────────────────────────────────────

// Plugin is implemented by every plugin.
type Plugin interface {
	Manifest() Manifest
	Execute(ctx context.Context, input Input) (*Output, error)
}

// Registry manages registered plugins.
type Registry interface {
	Register(p Plugin) error
	Unregister(id string) error
	Get(id string) (Plugin, bool)
	List() []Manifest
	Execute(ctx context.Context, id string, input Input) (*Output, error)
}

// ─── Base registry ────────────────────────────────────────────────────────────

type MemoryRegistry struct {
	plugins map[string]Plugin
}

func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{plugins: make(map[string]Plugin)}
}

func (r *MemoryRegistry) Register(p Plugin) error {
	m := p.Manifest()
	r.plugins[m.ID] = p
	return nil
}

func (r *MemoryRegistry) Unregister(id string) error {
	delete(r.plugins, id)
	return nil
}

func (r *MemoryRegistry) Get(id string) (Plugin, bool) {
	p, ok := r.plugins[id]
	return p, ok
}

func (r *MemoryRegistry) List() []Manifest {
	out := make([]Manifest, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p.Manifest())
	}
	return out
}

func (r *MemoryRegistry) Execute(ctx context.Context, id string, input Input) (*Output, error) {
	p, ok := r.plugins[id]
	if !ok {
		return nil, &PluginNotFoundError{ID: id}
	}
	start := time.Now()
	out, err := p.Execute(ctx, input)
	if out != nil {
		out.Latency = time.Since(start)
	}
	return out, err
}

type PluginNotFoundError struct{ ID string }

func (e *PluginNotFoundError) Error() string {
	return "plugin not found: " + e.ID
}
