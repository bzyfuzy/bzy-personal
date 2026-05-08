package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type WorkflowStatus string

const (
	WorkflowPending   WorkflowStatus = "pending"
	WorkflowRunning   WorkflowStatus = "running"
	WorkflowCompleted WorkflowStatus = "completed"
	WorkflowFailed    WorkflowStatus = "failed"
	WorkflowCancelled WorkflowStatus = "cancelled"
)

type WorkflowDef struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Version     int             `json:"version"`
	Definition  json.RawMessage `json:"definition"`
	CreatedAt   time.Time       `json:"created_at"`
}

type WorkflowRun struct {
	ID         string          `json:"id"`
	DefID      string          `json:"def_id"`
	Status     WorkflowStatus  `json:"status"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

type CreateWorkflowRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Definition  json.RawMessage `json:"definition"`
}

type RunWorkflowRequest struct {
	Input json.RawMessage `json:"input,omitempty"`
}

// ─── WorkflowsClient ──────────────────────────────────────────────────────────

type WorkflowsClient struct{ c *Client }

// Create registers a new workflow definition.
func (w *WorkflowsClient) Create(ctx context.Context, req CreateWorkflowRequest) (*WorkflowDef, error) {
	var def WorkflowDef
	if err := w.c.request(ctx, http.MethodPost, "/api/v1/workflows/", req, &def); err != nil {
		return nil, fmt.Errorf("workflow create: %w", err)
	}
	return &def, nil
}

// Run starts an execution of a workflow definition.
func (w *WorkflowsClient) Run(ctx context.Context, defID string, input any) (*WorkflowRun, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var run WorkflowRun
	if err := w.c.request(ctx, http.MethodPost,
		"/api/v1/workflows/"+defID+"/run",
		RunWorkflowRequest{Input: raw}, &run); err != nil {
		return nil, fmt.Errorf("workflow run: %w", err)
	}
	return &run, nil
}

// GetRun retrieves the state of a workflow run.
func (w *WorkflowsClient) GetRun(ctx context.Context, runID string) (*WorkflowRun, error) {
	var run WorkflowRun
	if err := w.c.request(ctx, http.MethodGet,
		"/api/v1/workflows/runs/"+runID, nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// Wait polls until the workflow run reaches a terminal state.
func (w *WorkflowsClient) Wait(ctx context.Context, runID string, interval time.Duration) (*WorkflowRun, error) {
	if interval == 0 {
		interval = 2 * time.Second
	}
	for {
		run, err := w.GetRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		switch run.Status {
		case WorkflowCompleted, WorkflowFailed, WorkflowCancelled:
			return run, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// OutputAs unmarshals the workflow run output into v.
func (r *WorkflowRun) OutputAs(v any) error {
	if r.Output == nil {
		return fmt.Errorf("workflow run has no output")
	}
	return json.Unmarshal(r.Output, v)
}
