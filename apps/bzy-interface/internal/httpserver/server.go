// Package httpserver wires Gin routes and starts the HTTP server.
package httpserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	ifconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-interface/config"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/gateway"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/middleware"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/streaming"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/ws"
	"github.com/bzyfuzy/bzy-personal/pkg/health"
)

// Server wraps the net/http server.
type Server struct {
	srv    *http.Server
	logger *zap.Logger
}

func New(cfg *ifconfig.Config, engine *gin.Engine, logger *zap.Logger) *Server {
	return &Server{
		srv: &http.Server{
			Addr:         cfg.HTTP.Addr,
			Handler:      engine,
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
		},
		logger: logger,
	}
}

func (s *Server) ListenAndServe() error {
	s.logger.Info("bzy-interface HTTP listening", zap.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return s.srv.Shutdown(ctx)
}

// RegisterRoutes builds the Gin router and attaches all handlers.
func RegisterRoutes(
	cfg *ifconfig.Config,
	mw *middleware.Bundle,
	brain *gateway.BrainClient,
	runner *gateway.RunnerClient,
	sseHub *streaming.Hub,
	wsHub *ws.Hub,
	checker *health.Checker,
	logger *zap.Logger,
) *gin.Engine {
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		middleware.CORS(cfg.HTTP.CORSOrigins),
		mw.RequestLogger(),
		mw.Tracing(cfg.ServiceName),
		middleware.ErrorHandler(),
		gin.Recovery(),
	)

	// ── Health ──────────────────────────────────────────────────────────────
	r.GET("/health/live", gin.WrapF(checker.LivenessHandler()))
	r.GET("/health/ready", gin.WrapF(checker.ReadinessHandler()))
	r.GET("/metrics", prometheusHandler())

	// ── API v1 ──────────────────────────────────────────────────────────────
	v1 := r.Group("/api/v1")

	// Auth (public)
	authGroup := v1.Group("/auth")
	{
		authGroup.POST("/login", handleLogin(brain))
		authGroup.POST("/refresh", handleRefresh(brain))
	}

	// Protected routes
	api := v1.Group("/", mw.RequireAuth(), mw.RateLimit())

	// Chat / AI
	chat := api.Group("/chat")
	{
		chat.POST("/completions", handleChatCompletion(brain))
		chat.GET("/stream", handleChatStream(brain, sseHub))
		chat.POST("/sessions", handleCreateSession(brain))
		chat.GET("/sessions/:id", handleGetSession(brain))
		chat.GET("/sessions/:id/messages", handleGetMessages(brain))
	}

	// Memory
	mem := api.Group("/memory")
	{
		mem.POST("/", handleStoreMemory(brain))
		mem.GET("/recall", handleRecallMemory(brain))
		mem.DELETE("/:id", handleDeleteMemory(brain))
	}

	// Documents / RAG
	docs := api.Group("/documents")
	{
		docs.POST("/", handleIngestDocument(brain))
		docs.POST("/query", handleRAGQuery(brain))
		docs.DELETE("/:id", handleDeleteDocument(brain))
	}

	// Agents
	agents := api.Group("/agents")
	{
		agents.GET("/", handleListAgents(brain))
		agents.POST("/:id/run", handleRunAgent(brain))
	}

	// Tasks
	tasks := api.Group("/tasks")
	{
		tasks.POST("/", handleEnqueueTask(runner))
		tasks.GET("/:id", handleGetTask(runner))
		tasks.GET("/:id/logs", handleTaskLogs(runner, sseHub))
		tasks.DELETE("/:id", handleCancelTask(runner))
	}

	// Workflows
	wf := api.Group("/workflows")
	{
		wf.POST("/", handleCreateWorkflow(runner))
		wf.POST("/:id/run", handleRunWorkflow(runner))
		wf.GET("/runs/:run_id", handleGetWorkflowRun(runner))
	}

	// WebSocket
	r.GET("/ws", mw.RequireAuth(), wsHub.Handler())

	return r
}

// ─── Placeholder handlers (replace with real implementations) ─────────────────

func handleLogin(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleRefresh(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleChatCompletion(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleChatStream(b *gateway.BrainClient, hub *streaming.Hub) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleCreateSession(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleGetSession(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleGetMessages(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleStoreMemory(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleRecallMemory(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleDeleteMemory(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleIngestDocument(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleRAGQuery(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleDeleteDocument(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleListAgents(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleRunAgent(b *gateway.BrainClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleEnqueueTask(r *gateway.RunnerClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleGetTask(r *gateway.RunnerClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleTaskLogs(r *gateway.RunnerClient, hub *streaming.Hub) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleCancelTask(r *gateway.RunnerClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleCreateWorkflow(r *gateway.RunnerClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleRunWorkflow(r *gateway.RunnerClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func handleGetWorkflowRun(r *gateway.RunnerClient) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"status": "todo"}) }
}
func prometheusHandler() gin.HandlerFunc {
	return func(c *gin.Context) { c.Status(http.StatusOK) }
}
