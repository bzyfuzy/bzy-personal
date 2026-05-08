// Package reasoning implements multi-step AI reasoning chains.
// Supports: ReAct (Reason+Act), Chain-of-Thought, Tree-of-Thought, and tool-augmented reasoning.
package reasoning

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/tools"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type ReasoningMode string

const (
	ModeReAct  ReasoningMode = "react"  // Reason → Act → Observe loop
	ModeCOT    ReasoningMode = "cot"    // Chain-of-Thought single pass
	ModeTOT    ReasoningMode = "tot"    // Tree-of-Thought (branching)
)

type Step struct {
	Thought    string         `json:"thought"`
	Action     string         `json:"action,omitempty"`
	ActionArgs map[string]any `json:"action_args,omitempty"`
	Observation string        `json:"observation,omitempty"`
	IsFinal    bool           `json:"is_final"`
}

type ReasoningTrace struct {
	ID        string    `json:"id"`
	Query     string    `json:"query"`
	Mode      ReasoningMode `json:"mode"`
	Steps     []Step    `json:"steps"`
	Answer    string    `json:"answer"`
	Latency   time.Duration `json:"latency"`
	TokensUsed int      `json:"tokens_used"`
}

// ─── Engine interface ─────────────────────────────────────────────────────────

type Engine interface {
	// Reason executes a reasoning loop and returns the final answer + trace.
	Reason(ctx context.Context, req ReasoningRequest) (*ReasoningTrace, error)
}

type ReasoningRequest struct {
	Query    string
	Mode     ReasoningMode
	Context  []providers.Message // prior conversation
	MaxSteps int
	Tools    []tools.Tool
}

// ─── ReAct engine ─────────────────────────────────────────────────────────────

type engine struct {
	provider    providers.ChatProvider
	toolExec    tools.Executor
	maxSteps    int
}

func NewEngine(reg providers.Registry, toolReg tools.Registry) Engine {
	return &engine{
		provider: reg.Default(),
		toolExec: tools.NewExecutor(toolReg),
		maxSteps: 10,
	}
}

func (e *engine) Reason(ctx context.Context, req ReasoningRequest) (*ReasoningTrace, error) {
	start := time.Now()
	trace := &ReasoningTrace{
		ID:    uuid.New().String(),
		Query: req.Query,
		Mode:  req.Mode,
	}

	if req.Mode == "" {
		req.Mode = ModeReAct
	}
	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = e.maxSteps
	}

	switch req.Mode {
	case ModeReAct:
		if err := e.runReAct(ctx, req, trace, maxSteps); err != nil {
			return trace, err
		}
	case ModeCOT:
		if err := e.runCOT(ctx, req, trace); err != nil {
			return trace, err
		}
	default:
		return nil, fmt.Errorf("unsupported reasoning mode: %s", req.Mode)
	}

	trace.Latency = time.Since(start)
	return trace, nil
}

func (e *engine) runReAct(ctx context.Context, req ReasoningRequest, trace *ReasoningTrace, maxSteps int) error {
	messages := buildReActMessages(req.Query, req.Context)
	toolSpecs := tools.ToProviderSpecs(req.Tools)

	for i := 0; i < maxSteps; i++ {
		resp, err := e.provider.Chat(ctx, providers.ChatRequest{
			Messages:    messages,
			Tools:       toolSpecs,
			Temperature: 0.1,
		})
		if err != nil {
			return fmt.Errorf("reasoning step %d: %w", i, err)
		}

		step := Step{}

		if resp.Finish == "tool_calls" && len(resp.Message.ToolCalls) > 0 {
			tc := resp.Message.ToolCalls[0]
			step.Thought = resp.Message.Content
			step.Action = tc.Name
			step.ActionArgs = map[string]any{"raw": tc.Arguments}

			obs, execErr := e.toolExec.Execute(ctx, tc.Name, tc.Arguments)
			if execErr != nil {
				obs = fmt.Sprintf("ERROR: %v", execErr)
			}
			step.Observation = obs

			messages = append(messages,
				providers.Message{Role: providers.RoleAssistant, Content: resp.Message.Content, ToolCalls: resp.Message.ToolCalls},
				providers.Message{Role: providers.RoleTool, Content: obs, ToolCallID: tc.ID},
			)
		} else {
			step.Thought = resp.Message.Content
			step.IsFinal = true
			trace.Answer = resp.Message.Content
			trace.Steps = append(trace.Steps, step)
			return nil
		}

		trace.Steps = append(trace.Steps, step)
	}
	return fmt.Errorf("reasoning exceeded max steps (%d)", maxSteps)
}

func (e *engine) runCOT(ctx context.Context, req ReasoningRequest, trace *ReasoningTrace) error {
	messages := []providers.Message{
		{Role: providers.RoleSystem, Content: "Think step by step before giving your final answer."},
	}
	messages = append(messages, req.Context...)
	messages = append(messages, providers.Message{Role: providers.RoleUser, Content: req.Query})

	resp, err := e.provider.Chat(ctx, providers.ChatRequest{
		Messages:    messages,
		Temperature: 0.2,
	})
	if err != nil {
		return err
	}
	trace.Steps = []Step{{Thought: resp.Message.Content, IsFinal: true}}
	trace.Answer = resp.Message.Content
	return nil
}

func buildReActMessages(query string, history []providers.Message) []providers.Message {
	sys := providers.Message{
		Role: providers.RoleSystem,
		Content: `You are a reasoning agent. For complex tasks:
1. Think about what you need to know
2. Use available tools to gather information
3. Continue reasoning until you have a complete answer
4. When ready, provide your final answer directly without using any tool.`,
	}
	msgs := []providers.Message{sys}
	msgs = append(msgs, history...)
	msgs = append(msgs, providers.Message{Role: providers.RoleUser, Content: query})
	return msgs
}
