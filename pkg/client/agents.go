package client

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type AgentDef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ToolIDs     []string `json:"tool_ids"`
	MaxSteps    int      `json:"max_steps"`
}

type RunAgentRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Input     string `json:"input"`
}

type RunAgentResponse struct {
	AgentRunID string        `json:"agent_run_id"`
	Answer     string        `json:"answer"`
	Steps      int           `json:"steps"`
	Latency    time.Duration `json:"latency_ms"`
}

// AgentStream is a live streaming agent execution.
type AgentStream struct {
	sse *SSEReader
}

// AgentChunk is one streamed step from the agent.
type AgentChunk struct {
	Delta    string `json:"delta"`
	Done     bool   `json:"done"`
	StepType string `json:"step_type"` // thought | action | observation | answer
}

// Text collects the complete agent answer, blocking until done.
func (s *AgentStream) Text(ctx context.Context) (string, error) {
	var full string
	err := s.sse.Iter(ctx, func(e *SSEEvent) error {
		var chunk AgentChunk
		if err := e.DecodeData(&chunk); err != nil {
			return err
		}
		if chunk.StepType == "answer" || chunk.StepType == "" {
			full += chunk.Delta
		}
		return nil
	})
	return full, err
}

// Steps iterates over all agent steps (thoughts, actions, observations, answer).
func (s *AgentStream) Steps(ctx context.Context, fn func(AgentChunk) error) error {
	return s.sse.Iter(ctx, func(e *SSEEvent) error {
		var chunk AgentChunk
		if err := e.DecodeData(&chunk); err != nil {
			return err
		}
		return fn(chunk)
	})
}

// Close releases the stream.
func (s *AgentStream) Close() { s.sse.Close() }

// ─── AgentsClient ─────────────────────────────────────────────────────────────

type AgentsClient struct{ c *Client }

// List returns all registered agent definitions.
func (a *AgentsClient) List(ctx context.Context) ([]AgentDef, error) {
	var resp struct {
		Agents []AgentDef `json:"agents"`
	}
	if err := a.c.request(ctx, http.MethodGet, "/api/v1/agents/", nil, &resp); err != nil {
		return nil, fmt.Errorf("agents list: %w", err)
	}
	return resp.Agents, nil
}

// Run executes an agent synchronously and returns the final answer.
func (a *AgentsClient) Run(ctx context.Context, agentID string, req RunAgentRequest) (*RunAgentResponse, error) {
	var resp RunAgentResponse
	if err := a.c.request(ctx, http.MethodPost,
		"/api/v1/agents/"+agentID+"/run", req, &resp); err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}
	return &resp, nil
}

// RunStream executes an agent with step-by-step SSE streaming.
func (a *AgentsClient) RunStream(ctx context.Context, agentID string, req RunAgentRequest) (*AgentStream, error) {
	resp, err := a.c.rawRequest(ctx, http.MethodPost,
		"/api/v1/agents/"+agentID+"/stream", req,
		map[string]string{"Accept": "text/event-stream"})
	if err != nil {
		return nil, fmt.Errorf("agent stream: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, a.c.decodeError(resp)
	}
	return &AgentStream{sse: newSSEReader(resp)}, nil
}

// Ask is a shortcut: runs the named agent with a plain question, returns the answer.
func (a *AgentsClient) Ask(ctx context.Context, agentID, question string) (string, error) {
	resp, err := a.Run(ctx, agentID, RunAgentRequest{Input: question})
	if err != nil {
		return "", err
	}
	return resp.Answer, nil
}
