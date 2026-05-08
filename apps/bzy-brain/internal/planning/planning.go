// Package planning decomposes high-level goals into executable task trees.
package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
)

// ─── Domain types ─────────────────────────────────────────────────────────────

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskRunning    TaskStatus = "running"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskSkipped    TaskStatus = "skipped"
	TaskBlocked    TaskStatus = "blocked"
)

// Task is a single unit of work in a plan.
type Task struct {
	ID           string         `json:"id"`
	PlanID       string         `json:"plan_id"`
	ParentID     string         `json:"parent_id,omitempty"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Type         string         `json:"type"`      // tool | agent | human | subplan
	ToolID       string         `json:"tool_id,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty"` // task IDs
	Status       TaskStatus     `json:"status"`
	Result       any            `json:"result,omitempty"`
	Error        string         `json:"error,omitempty"`
	Order        int            `json:"order"`
	Children     []*Task        `json:"children,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
}

// Plan is a tree of tasks representing a decomposed goal.
type Plan struct {
	ID          string    `json:"id"`
	Goal        string    `json:"goal"`
	Tasks       []*Task   `json:"tasks"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ─── Planner interface ────────────────────────────────────────────────────────

type Planner interface {
	// Decompose generates a plan for a given goal.
	Decompose(ctx context.Context, goal string, context []providers.Message) (*Plan, error)
	// Replan revises a plan given new observations or failures.
	Replan(ctx context.Context, planID string, observation string) (*Plan, error)
	// NextTask returns the next executable task (respects dependencies).
	NextTask(ctx context.Context, plan *Plan) (*Task, error)
	// UpdateTask persists a task status change.
	UpdateTask(ctx context.Context, planID, taskID string, status TaskStatus, result any) error
}

// ─── LLM-based planner ────────────────────────────────────────────────────────

type llmPlanner struct {
	provider providers.ChatProvider
}

func NewPlanner(reg providers.Registry) Planner {
	return &llmPlanner{provider: reg.Default()}
}

func (p *llmPlanner) Decompose(ctx context.Context, goal string, context []providers.Message) (*Plan, error) {
	prompt := fmt.Sprintf(`Decompose the following goal into a JSON task plan.
Output ONLY valid JSON matching this schema:
{
  "tasks": [
    {
      "title": "string",
      "description": "string",
      "type": "tool|agent|human",
      "tool_id": "string (optional)",
      "dependencies": ["task_title_1"],
      "order": 0
    }
  ]
}

Goal: %s`, goal)

	messages := []providers.Message{
		{Role: providers.RoleSystem, Content: "You are a task planning system. Output only valid JSON."},
	}
	messages = append(messages, context...)
	messages = append(messages, providers.Message{Role: providers.RoleUser, Content: prompt})

	resp, err := p.provider.Chat(ctx, providers.ChatRequest{
		Messages:    messages,
		Temperature: 0.0,
	})
	if err != nil {
		return nil, fmt.Errorf("plan decompose: %w", err)
	}

	var raw struct {
		Tasks []struct {
			Title       string         `json:"title"`
			Description string         `json:"description"`
			Type        string         `json:"type"`
			ToolID      string         `json:"tool_id"`
			Dependencies []string      `json:"dependencies"`
			Order       int            `json:"order"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(resp.Message.Content), &raw); err != nil {
		return nil, fmt.Errorf("plan parse: %w", err)
	}

	plan := &Plan{
		ID:        uuid.New().String(),
		Goal:      goal,
		Status:    TaskPending,
		CreatedAt: time.Now().UTC(),
	}

	for _, t := range raw.Tasks {
		plan.Tasks = append(plan.Tasks, &Task{
			ID:           uuid.New().String(),
			PlanID:       plan.ID,
			Title:        t.Title,
			Description:  t.Description,
			Type:         t.Type,
			ToolID:       t.ToolID,
			Dependencies: t.Dependencies,
			Status:       TaskPending,
			Order:        t.Order,
			CreatedAt:    time.Now().UTC(),
		})
	}
	return plan, nil
}

func (p *llmPlanner) Replan(ctx context.Context, planID, observation string) (*Plan, error) {
	// Fetch existing plan and re-generate with observation context
	return nil, nil // implementation stores plans in DB
}

func (p *llmPlanner) NextTask(_ context.Context, plan *Plan) (*Task, error) {
	completed := map[string]bool{}
	for _, t := range plan.Tasks {
		if t.Status == TaskCompleted {
			completed[t.ID] = true
		}
	}

	for _, t := range plan.Tasks {
		if t.Status != TaskPending {
			continue
		}
		ready := true
		for _, dep := range t.Dependencies {
			if !completed[dep] {
				ready = false
				break
			}
		}
		if ready {
			return t, nil
		}
	}
	return nil, nil // all done or blocked
}

func (p *llmPlanner) UpdateTask(_ context.Context, _, taskID string, status TaskStatus, result any) error {
	// Persist to DB
	return nil
}
