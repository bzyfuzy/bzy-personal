// Package main shows idiomatic usage of the bzy-personal API client.
// Run: go run ./examples/client_usage.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bzyfuzy/bzy-personal/pkg/client"
)

func main() {
	ctx := context.Background()

	// ── 1. Create client ──────────────────────────────────────────────────────
	c := client.New("http://localhost:8080",
		client.WithAPIKey("your-api-key-here"),
		client.WithTimeout(60*time.Second),
		client.WithMaxRetries(3),
	)

	// ── 2. JWT login (alternative to API key) ─────────────────────────────────
	loginExample(ctx, c)

	// ── 3. Chat completions ───────────────────────────────────────────────────
	chatExample(ctx, c)

	// ── 4. Streaming chat ─────────────────────────────────────────────────────
	chatStreamExample(ctx, c)

	// ── 5. Memory ─────────────────────────────────────────────────────────────
	memoryExample(ctx, c)

	// ── 6. Document ingestion + RAG ───────────────────────────────────────────
	ragExample(ctx, c)

	// ── 7. Agents ─────────────────────────────────────────────────────────────
	agentExample(ctx, c)

	// ── 8. Tasks + live logs ──────────────────────────────────────────────────
	tasksExample(ctx, c)

	// ── 9. Workflows ─────────────────────────────────────────────────────────
	workflowExample(ctx, c)

	// ── 10. WebSocket realtime ───────────────────────────────────────────────
	websocketExample(ctx, c)
}

// ─── Login ────────────────────────────────────────────────────────────────────

func loginExample(ctx context.Context, c *client.Client) {
	pair, err := c.Auth.Login(ctx, "user@example.com", "secret")
	if err != nil {
		log.Printf("login failed: %v", err)
		return
	}
	fmt.Printf("Logged in — token expires: %s\n", pair.ExpiresAt.Format(time.RFC3339))
	// Token is stored automatically; subsequent calls use it.
}

// ─── Chat completions ─────────────────────────────────────────────────────────

func chatExample(ctx context.Context, c *client.Client) {
	resp, err := c.Chat.Complete(ctx, client.ChatRequest{
		Messages: []client.Message{
			{Role: client.RoleSystem, Content: "You are a concise assistant."},
			{Role: client.RoleUser, Content: "What is pgvector used for?"},
		},
		Model:       "gpt-4o",
		Temperature: 0.3,
	})
	if err != nil {
		log.Printf("chat: %v", err)
		return
	}
	fmt.Printf("Chat response: %s\n", resp.Message.Content)
	fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)
}

// ─── Streaming chat ───────────────────────────────────────────────────────────

func chatStreamExample(ctx context.Context, c *client.Client) {
	stream, err := c.Chat.AskStream(ctx, "", "Explain embeddings in one paragraph.")
	if err != nil {
		log.Printf("stream: %v", err)
		return
	}
	defer stream.Close()

	fmt.Print("Streaming: ")
	err = stream.Iter(ctx, func(delta client.ChatDelta) error {
		fmt.Print(delta.Delta)
		return nil
	})
	fmt.Println()
	if err != nil {
		log.Printf("stream error: %v", err)
	}
}

// ─── Memory ───────────────────────────────────────────────────────────────────

func memoryExample(ctx context.Context, c *client.Client) {
	// Store a semantic memory
	id, err := c.Memory.Store(ctx, client.StoreMemoryRequest{
		Type:       client.MemorySemantic,
		Content:    "User prefers concise answers with code examples.",
		Importance: 0.9,
		Metadata:   map[string]any{"source": "user_feedback"},
	})
	if err != nil {
		log.Printf("memory store: %v", err)
		return
	}
	fmt.Printf("Stored memory: %s\n", id)

	// Recall memories related to a topic
	results, err := c.Memory.Recall(ctx, "user preferences", 5, 0.7)
	if err != nil {
		log.Printf("memory recall: %v", err)
		return
	}
	fmt.Printf("Recalled %d memories:\n", len(results))
	for _, r := range results {
		fmt.Printf("  [%.2f] %s\n", r.Score, r.Memory.Content)
	}

	// Forget a specific memory
	if err := c.Memory.Forget(ctx, id); err != nil {
		log.Printf("memory forget: %v", err)
	}
}

// ─── Document ingestion + RAG ─────────────────────────────────────────────────

func ragExample(ctx context.Context, c *client.Client) {
	// Ingest a document
	ingestResp, err := c.Documents.Ingest(ctx, client.IngestRequest{
		Title:         "Go Concurrency Patterns",
		Content:       `Go uses goroutines and channels for concurrency...`,
		Source:        "https://go.dev/blog/pipelines",
		ChunkStrategy: "paragraph",
		ChunkSize:     512,
		ChunkOverlap:  64,
	})
	if err != nil {
		log.Printf("ingest: %v", err)
		return
	}
	fmt.Printf("Ingested: %s (%d chunks)\n", ingestResp.DocumentID, ingestResp.ChunkCount)

	// RAG query
	qResp, err := c.Documents.Query(ctx, client.RAGQueryRequest{
		Query: "How do Go channels work?",
		TopK:  3,
		History: []client.Message{
			{Role: client.RoleUser, Content: "I'm learning Go concurrency."},
		},
	})
	if err != nil {
		log.Printf("rag query: %v", err)
		return
	}
	fmt.Printf("RAG answer (%dms): %s\n", qResp.LatencyMs, qResp.Response)
	fmt.Printf("Supporting chunks: %d\n", len(qResp.Chunks))
}

// ─── Agents ───────────────────────────────────────────────────────────────────

func agentExample(ctx context.Context, c *client.Client) {
	// List available agents
	agents, err := c.Agents.List(ctx)
	if err != nil {
		log.Printf("agents list: %v", err)
		return
	}
	fmt.Printf("Available agents: %d\n", len(agents))
	for _, a := range agents {
		fmt.Printf("  %s: %s\n", a.ID, a.Description)
	}

	// Run the researcher agent with streaming steps
	stream, err := c.Agents.RunStream(ctx, "researcher", client.RunAgentRequest{
		Input: "Find the top 3 benefits of using pgvector over a dedicated vector DB.",
	})
	if err != nil {
		log.Printf("agent stream: %v", err)
		return
	}

	fmt.Println("\nAgent steps:")
	err = stream.Steps(ctx, func(chunk client.AgentChunk) error {
		if chunk.StepType != "" {
			fmt.Printf("  [%s] %s\n", chunk.StepType, chunk.Delta)
		}
		return nil
	})
	if err != nil {
		log.Printf("agent steps: %v", err)
	}
}

// ─── Tasks ────────────────────────────────────────────────────────────────────

func tasksExample(ctx context.Context, c *client.Client) {
	// Enqueue a background task
	taskID, err := c.Tasks.EnqueueJSON(ctx, "email.send", map[string]any{
		"to":      "user@example.com",
		"subject": "Daily digest",
		"body":    "Here are your action items for today...",
	}, func(req *client.EnqueueRequest) {
		req.Priority = client.PriorityNormal
		req.MaxAttempts = 3
	})
	if err != nil {
		log.Printf("enqueue: %v", err)
		return
	}
	fmt.Printf("Enqueued task: %s\n", taskID)

	// Stream live logs
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logs, err := c.Tasks.Logs(ctx2, taskID, true)
	if err != nil {
		log.Printf("task logs: %v", err)
		return
	}
	fmt.Println("Task logs:")
	_ = logs.Each(ctx2, func(entry *client.LogEntry) error {
		fmt.Printf("  [%s] %s\n", entry.Level, entry.Message)
		return nil
	})

	// Wait for completion
	task, err := c.Tasks.Wait(ctx, taskID, time.Second)
	if err != nil {
		log.Printf("task wait: %v", err)
		return
	}
	fmt.Printf("Task finished: %s\n", task.Status)
}

// ─── Workflows ────────────────────────────────────────────────────────────────

func workflowExample(ctx context.Context, c *client.Client) {
	// Run a pre-registered workflow
	run, err := c.Workflows.Run(ctx, "news-digest", map[string]any{
		"date": time.Now().Format("2006-01-02"),
	})
	if err != nil {
		log.Printf("workflow run: %v", err)
		return
	}
	fmt.Printf("Workflow run started: %s\n", run.ID)

	// Poll until done
	finalRun, err := c.Workflows.Wait(ctx, run.ID, 2*time.Second)
	if err != nil {
		log.Printf("workflow wait: %v", err)
		return
	}
	fmt.Printf("Workflow status: %s\n", finalRun.Status)

	// Decode the output
	var output map[string]any
	if err := finalRun.OutputAs(&output); err == nil {
		fmt.Printf("Workflow output keys: %v\n", outputKeys(output))
	}
}

// ─── WebSocket ────────────────────────────────────────────────────────────────

func websocketExample(ctx context.Context, c *client.Client) {
	conn, err := c.Connect(ctx)
	if err != nil {
		log.Printf("ws connect: %v", err)
		return
	}
	defer conn.Close()

	// React to task updates
	conn.On(client.WSTypeTaskUpdate, func(msg client.WSMessage) error {
		var task client.Task
		if err := msg.DecodePayload(&task); err != nil {
			return err
		}
		fmt.Printf("Task update: %s → %s\n", task.ID, task.Status)
		return nil
	})

	// React to agent responses
	conn.On(client.WSTypeAgentResponse, func(msg client.WSMessage) error {
		var chunk client.AgentChunk
		if err := msg.DecodePayload(&chunk); err != nil {
			return err
		}
		fmt.Printf("Agent: %s\n", chunk.Delta)
		return nil
	})

	fmt.Println("WebSocket connected — waiting for events (30s)...")
	select {
	case <-conn.Done():
		fmt.Println("Connection closed by server")
	case <-time.After(30 * time.Second):
		fmt.Println("Demo timeout")
	}
}

func outputKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
