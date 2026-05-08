// Package agents manages AI agent definitions, spawning, and lifecycle.
package agents

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/tools"
)

// ─── Agent model ──────────────────────────────────────────────────────────────

type AgentStatus string

const (
	StatusIdle     AgentStatus = "idle"
	StatusRunning  AgentStatus = "running"
	StatusWaiting  AgentStatus = "waiting"
	StatusFinished AgentStatus = "finished"
	StatusFailed   AgentStatus = "failed"
)

// AgentDef is a static definition of an agent's persona, tools, and capabilities.
type AgentDef struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"system_prompt"`
	Model        string   `json:"model,omitempty"`
	ToolIDs      []string `json:"tool_ids"`
	MaxSteps     int      `json:"max_steps"`
	Temperature  float64  `json:"temperature"`
}

// AgentState is a live instance of a running agent.
type AgentState struct {
	ID         string         `json:"id"`
	DefID      string         `json:"def_id"`
	SessionID  string         `json:"session_id"`
	Status     AgentStatus    `json:"status"`
	Messages   []providers.Message `json:"messages"`
	StepCount  int            `json:"step_count"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	SpawnedAt  time.Time      `json:"spawned_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

// RunResult holds the final agent output.
type RunResult struct {
	AgentID  string            `json:"agent_id"`
	Answer   string            `json:"answer"`
	Steps    int               `json:"steps"`
	Latency  time.Duration     `json:"latency"`
}

// ─── Interfaces ───────────────────────────────────────────────────────────────

// Agent is a stateful, tool-using AI actor.
type Agent interface {
	ID() string
	Def() AgentDef
	Run(ctx context.Context, input string) (*RunResult, error)
	State() *AgentState
}

// Registry manages available agent definitions.
type Registry interface {
	Register(def AgentDef) error
	Get(id string) (AgentDef, bool)
	List() []AgentDef
	// Spawn creates a live agent instance from a definition.
	Spawn(ctx context.Context, defID, sessionID string) (Agent, error)
}

// ─── Registry implementation ──────────────────────────────────────────────────

type registry struct {
	defs      map[string]AgentDef
	providers providers.Registry
	tools     tools.Registry
}

func NewRegistry(provReg providers.Registry, toolReg tools.Registry) Registry {
	r := &registry{
		defs:      make(map[string]AgentDef),
		providers: provReg,
		tools:     toolReg,
	}
	r.registerBuiltins()
	return r
}

func (r *registry) Register(def AgentDef) error {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	r.defs[def.ID] = def
	return nil
}

func (r *registry) Get(id string) (AgentDef, bool) {
	d, ok := r.defs[id]
	return d, ok
}

func (r *registry) List() []AgentDef {
	out := make([]AgentDef, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
	return out
}

func (r *registry) Spawn(ctx context.Context, defID, sessionID string) (Agent, error) {
	def, ok := r.defs[defID]
	if !ok {
		return nil, &AgentNotFoundError{ID: defID}
	}

	agentTools := make([]tools.Tool, 0, len(def.ToolIDs))
	for _, tid := range def.ToolIDs {
		if t, ok := r.tools.Get(tid); ok {
			agentTools = append(agentTools, t)
		}
	}

	state := &AgentState{
		ID:        uuid.New().String(),
		DefID:     defID,
		SessionID: sessionID,
		Status:    StatusIdle,
		SpawnedAt: time.Now().UTC(),
	}

	return &liveAgent{
		def:      def,
		state:    state,
		provider: r.providers.Default(),
		tools:    agentTools,
	}, nil
}

func (r *registry) registerBuiltins() {
	_ = r.Register(AgentDef{
		ID:           "general",
		Name:         "General Assistant",
		Description:  "A general-purpose AI assistant with access to core tools",
		SystemPrompt: "You are a helpful personal AI assistant. Be concise, accurate, and proactive.",
		MaxSteps:     15,
		Temperature:  0.7,
	})
	_ = r.Register(AgentDef{
		ID:           "researcher",
		Name:         "Research Agent",
		Description:  "Deep research agent that searches, reads, and synthesizes information",
		SystemPrompt: "You are a research specialist. Gather comprehensive information and synthesize clear summaries.",
		MaxSteps:     20,
		Temperature:  0.2,
	})
}

// ─── Live agent ───────────────────────────────────────────────────────────────

type liveAgent struct {
	def      AgentDef
	state    *AgentState
	provider providers.ChatProvider
	tools    []tools.Tool
}

func (a *liveAgent) ID() string          { return a.state.ID }
func (a *liveAgent) Def() AgentDef       { return a.def }
func (a *liveAgent) State() *AgentState  { return a.state }

func (a *liveAgent) Run(ctx context.Context, input string) (*RunResult, error) {
	start := time.Now()
	a.state.Status = StatusRunning

	sysmsg := providers.Message{Role: providers.RoleSystem, Content: a.def.SystemPrompt}
	a.state.Messages = []providers.Message{sysmsg}
	a.state.Messages = append(a.state.Messages,
		providers.Message{Role: providers.RoleUser, Content: input})

	toolSpecs := tools.ToProviderSpecs(a.tools)
	maxSteps := a.def.MaxSteps
	if maxSteps == 0 {
		maxSteps = 10
	}

	for step := 0; step < maxSteps; step++ {
		resp, err := a.provider.Chat(ctx, providers.ChatRequest{
			Messages:    a.state.Messages,
			Tools:       toolSpecs,
			Temperature: a.def.Temperature,
		})
		if err != nil {
			a.state.Status = StatusFailed
			return nil, err
		}

		a.state.Messages = append(a.state.Messages, resp.Message)
		a.state.StepCount++

		if resp.Finish == "stop" || len(resp.Message.ToolCalls) == 0 {
			now := time.Now().UTC()
			a.state.Status = StatusFinished
			a.state.FinishedAt = &now
			return &RunResult{
				AgentID: a.state.ID,
				Answer:  resp.Message.Content,
				Steps:   step + 1,
				Latency: time.Since(start),
			}, nil
		}

		// Process tool calls
		for _, tc := range resp.Message.ToolCalls {
			obs := a.executeTool(ctx, tc.Name, tc.Arguments)
			a.state.Messages = append(a.state.Messages, providers.Message{
				Role:       providers.RoleTool,
				Content:    obs,
				ToolCallID: tc.ID,
			})
		}
	}

	a.state.Status = StatusFailed
	return nil, &MaxStepsExceededError{Steps: maxSteps}
}

func (a *liveAgent) executeTool(ctx context.Context, name, args string) string {
	for _, t := range a.tools {
		if t.Spec().Name == name {
			result, err := t.Execute(ctx, args)
			if err != nil {
				return "error: " + err.Error()
			}
			return result
		}
	}
	return "tool not found: " + name
}

type AgentNotFoundError struct{ ID string }

func (e *AgentNotFoundError) Error() string { return "agent def not found: " + e.ID }

type MaxStepsExceededError struct{ Steps int }

func (e *MaxStepsExceededError) Error() string {
	return fmt.Sprintf("agent exceeded max steps: %d", e.Steps)
}

import "fmt"
