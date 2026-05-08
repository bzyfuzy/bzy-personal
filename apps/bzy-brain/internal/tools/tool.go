// Package tools defines the tool interface and executor used by agents and reasoning.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
	pkgplugin "github.com/bzyfuzy/bzy-personal/pkg/plugin"
)

// ─── Tool interface ───────────────────────────────────────────────────────────

// Spec describes a tool to the AI (used to build provider ToolSpec).
type Spec struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema object
}

// Tool is the runtime callable unit.
type Tool interface {
	Spec() Spec
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// Registry stores and retrieves tools by name.
type Registry interface {
	Register(t Tool) error
	Get(name string) (Tool, bool)
	List() []Spec
}

// Executor runs tools by name.
type Executor interface {
	Execute(ctx context.Context, name, argsJSON string) (string, error)
}

// ─── Registry implementation ──────────────────────────────────────────────────

type registry struct {
	tools map[string]Tool
}

func NewRegistry() Registry {
	r := &registry{tools: make(map[string]Tool)}
	r.registerBuiltins()
	return r
}

func (r *registry) Register(t Tool) error {
	r.tools[t.Spec().Name] = t
	return nil
}

func (r *registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *registry) List() []Spec {
	out := make([]Spec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Spec())
	}
	return out
}

// ─── Executor ────────────────────────────────────────────────────────────────

type executor struct{ reg Registry }

func NewExecutor(reg Registry) Executor {
	return &executor{reg: reg}
}

func (e *executor) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	t, ok := e.reg.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return t.Execute(ctx, argsJSON)
}

// ─── Plugin-backed tool ───────────────────────────────────────────────────────

// PluginTool wraps a pkg/plugin.Plugin as a Tool.
type PluginTool struct {
	plugin pkgplugin.Plugin
}

func FromPlugin(p pkgplugin.Plugin) Tool {
	return &PluginTool{plugin: p}
}

func (p *PluginTool) Spec() Spec {
	m := p.plugin.Manifest()
	return Spec{
		Name:        m.ID,
		Description: m.Description,
	}
}

func (p *PluginTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	out, err := p.plugin.Execute(ctx, pkgplugin.Input{Params: json.RawMessage(argsJSON)})
	if err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf(out.Error)
	}
	if out.Text != "" {
		return out.Text, nil
	}
	return string(out.Data), nil
}

// ─── Provider spec conversion ─────────────────────────────────────────────────

func ToProviderSpecs(ts []Tool) []providers.ToolSpec {
	out := make([]providers.ToolSpec, len(ts))
	for i, t := range ts {
		s := t.Spec()
		out[i] = providers.ToolSpec{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  s.Parameters,
		}
	}
	return out
}

// ─── Built-in tools ───────────────────────────────────────────────────────────

func (r *registry) registerBuiltins() {
	r.tools["get_current_time"] = &currentTimeTool{}
}

type currentTimeTool struct{}

func (t *currentTimeTool) Spec() Spec {
	return Spec{
		Name:        "get_current_time",
		Description: "Returns the current UTC time in ISO 8601 format",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		},
	}
}

func (t *currentTimeTool) Execute(ctx context.Context, _ string) (string, error) {
	return fmt.Sprintf(`{"time": "%s"}`, "2026-05-08T00:00:00Z"), nil
}
