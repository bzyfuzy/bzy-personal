// Package workflows orchestrates multi-step AI workflow execution.
// A workflow is a directed acyclic graph (DAG) of nodes: agent calls, tool calls,
// conditions, loops, and sub-workflows.
package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ─── Domain types ─────────────────────────────────────────────────────────────

type NodeType string

const (
	NodeAgent     NodeType = "agent"
	NodeTool      NodeType = "tool"
	NodeCondition NodeType = "condition"
	NodeTransform NodeType = "transform"
	NodeWait      NodeType = "wait"
	NodeSubflow   NodeType = "subflow"
	NodeInput     NodeType = "input"
	NodeOutput    NodeType = "output"
)

type WorkflowStatus string

const (
	WFPending   WorkflowStatus = "pending"
	WFRunning   WorkflowStatus = "running"
	WFCompleted WorkflowStatus = "completed"
	WFFailed    WorkflowStatus = "failed"
	WFCancelled WorkflowStatus = "cancelled"
)

// Node is a single step in a workflow graph.
type Node struct {
	ID         string         `json:"id"`
	Type       NodeType       `json:"type"`
	Name       string         `json:"name"`
	Config     map[string]any `json:"config"`
	Next       []string       `json:"next"`        // node IDs for normal flow
	OnError    string         `json:"on_error,omitempty"`
	OnSuccess  string         `json:"on_success,omitempty"`
}

// WorkflowDef is the static workflow definition (stored, versioned).
type WorkflowDef struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Version     int              `json:"version"`
	Nodes       map[string]*Node `json:"nodes"`
	EntryNode   string           `json:"entry_node"`
	InputSchema map[string]any   `json:"input_schema,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
}

// WorkflowRun is an active execution instance.
type WorkflowRun struct {
	ID         string         `json:"id"`
	DefID      string         `json:"def_id"`
	Status     WorkflowStatus `json:"status"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output,omitempty"`
	NodeStates map[string]*NodeState `json:"node_states"`
	Error      string         `json:"error,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

// NodeState tracks execution state for a single node within a run.
type NodeState struct {
	NodeID      string         `json:"node_id"`
	Status      WorkflowStatus `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	Attempts    int            `json:"attempts"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
}

// ─── Engine interface ─────────────────────────────────────────────────────────

type Engine interface {
	// RegisterDef stores a workflow definition.
	RegisterDef(ctx context.Context, def *WorkflowDef) error
	// GetDef retrieves a workflow definition by ID.
	GetDef(ctx context.Context, id string) (*WorkflowDef, error)
	// Execute starts a workflow run and returns the run ID.
	Execute(ctx context.Context, defID string, input json.RawMessage) (*WorkflowRun, error)
	// GetRun retrieves a run by ID.
	GetRun(ctx context.Context, runID string) (*WorkflowRun, error)
	// Cancel stops a running workflow.
	Cancel(ctx context.Context, runID string) error
}

// NodeHandler executes a single node type.
type NodeHandler interface {
	Type() NodeType
	Handle(ctx context.Context, node *Node, input json.RawMessage, run *WorkflowRun) (json.RawMessage, error)
}

// ─── Engine implementation ────────────────────────────────────────────────────

type engine struct {
	defs     map[string]*WorkflowDef
	runs     map[string]*WorkflowRun
	handlers map[NodeType]NodeHandler
}

func NewEngine() Engine {
	return &engine{
		defs:     make(map[string]*WorkflowDef),
		runs:     make(map[string]*WorkflowRun),
		handlers: make(map[NodeType]NodeHandler),
	}
}

func (e *engine) RegisterDef(_ context.Context, def *WorkflowDef) error {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	def.CreatedAt = time.Now().UTC()
	e.defs[def.ID] = def
	return nil
}

func (e *engine) GetDef(_ context.Context, id string) (*WorkflowDef, error) {
	d, ok := e.defs[id]
	if !ok {
		return nil, fmt.Errorf("workflow def not found: %s", id)
	}
	return d, nil
}

func (e *engine) Execute(ctx context.Context, defID string, input json.RawMessage) (*WorkflowRun, error) {
	def, err := e.GetDef(ctx, defID)
	if err != nil {
		return nil, err
	}

	run := &WorkflowRun{
		ID:         uuid.New().String(),
		DefID:      defID,
		Status:     WFRunning,
		Input:      input,
		NodeStates: make(map[string]*NodeState),
		StartedAt:  time.Now().UTC(),
	}
	e.runs[run.ID] = run

	go func() {
		if err := e.executeDAG(context.Background(), def, run, input); err != nil {
			run.Status = WFFailed
			run.Error = err.Error()
		}
	}()

	return run, nil
}

func (e *engine) executeDAG(ctx context.Context, def *WorkflowDef, run *WorkflowRun, input json.RawMessage) error {
	current := def.EntryNode
	payload := input

	for current != "" {
		node, ok := def.Nodes[current]
		if !ok {
			return fmt.Errorf("node not found: %s", current)
		}

		ns := &NodeState{NodeID: current, Status: WFRunning, Input: payload, StartedAt: time.Now().UTC()}
		run.NodeStates[current] = ns

		h, ok := e.handlers[node.Type]
		if !ok {
			return fmt.Errorf("no handler for node type: %s", node.Type)
		}

		output, err := h.Handle(ctx, node, payload, run)
		now := time.Now().UTC()
		ns.FinishedAt = &now

		if err != nil {
			ns.Status = WFFailed
			ns.Error = err.Error()
			if node.OnError != "" {
				current = node.OnError
				continue
			}
			return err
		}

		ns.Status = WFCompleted
		ns.Output = output
		payload = output

		if len(node.Next) == 0 {
			break
		}
		current = node.Next[0]
	}

	now := time.Now().UTC()
	run.Status = WFCompleted
	run.Output = payload
	run.FinishedAt = &now
	return nil
}

func (e *engine) GetRun(_ context.Context, runID string) (*WorkflowRun, error) {
	r, ok := e.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	return r, nil
}

func (e *engine) Cancel(_ context.Context, runID string) error {
	r, ok := e.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	r.Status = WFCancelled
	return nil
}
