// Package httpserver wires Gin routes and starts the HTTP server.
package httpserver

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	ifconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-interface/config"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/auth"
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
	db *sql.DB,
	jwtSvc *auth.JWTService,
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
		authGroup.POST("/login", handleLogin(db, jwtSvc))
		authGroup.POST("/refresh", handleRefresh(jwtSvc))
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
		mem.GET("/", handleListMemory(brain))
		mem.DELETE("/:id", handleDeleteMemory(brain))
	}

	// Documents / RAG
	docs := api.Group("/documents")
	{
		docs.POST("/", handleIngestDocument(brain))
		docs.POST("/query", handleRAGQuery(brain))
		docs.GET("/query/stream", handleRAGQueryStream(brain, sseHub))
		docs.DELETE("/:id", handleDeleteDocument(brain))
	}

	// Agents
	agents := api.Group("/agents")
	{
		agents.GET("/", handleListAgents(brain))
		agents.POST("/:id/run", handleRunAgent(brain))
		agents.GET("/:id/stream", handleRunAgentStream(brain, sseHub))
	}

	// Tasks
	tasks := api.Group("/tasks")
	{
		tasks.POST("/", handleEnqueueTask(runner))
		tasks.GET("/", handleListTasks(runner))
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

// ─── Auth handlers ─────────────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func handleLogin(db *sql.DB, jwtSvc *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": err.Error()})
			return
		}

		var userID, email string
		var roles []string
		err := db.QueryRowContext(c.Request.Context(), `
			SELECT id, email, roles
			FROM   users
			WHERE  email         = $1
			  AND  password_hash = crypt($2, password_hash)
			  AND  is_active     = TRUE`,
			req.Email, req.Password,
		).Scan(&userID, &email, (*pqStringArray)(&roles))

		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_CREDENTIALS", "message": "invalid email or password"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL", "message": "database error"})
			return
		}

		pair, err := jwtSvc.Issue(userID, email, roles)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL", "message": "could not issue token"})
			return
		}
		c.JSON(http.StatusOK, pair)
	}
}

func handleRefresh(jwtSvc *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": err.Error()})
			return
		}
		claims, err := jwtSvc.Validate(body.RefreshToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_TOKEN", "message": "invalid or expired refresh token"})
			return
		}
		pair, err := jwtSvc.Issue(claims.UserID, claims.Email, claims.Roles)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL", "message": "could not issue token"})
			return
		}
		c.JSON(http.StatusOK, pair)
	}
}

// pqStringArray scans a Postgres TEXT[] column into a []string.
type pqStringArray []string

func (a *pqStringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	b, ok := src.([]byte)
	if !ok {
		s, ok2 := src.(string)
		if !ok2 {
			return fmt.Errorf("pqStringArray: unexpected type %T", src)
		}
		b = []byte(s)
	}
	// Postgres array literal: {admin,user}
	s := string(b)
	s = s[1 : len(s)-1] // strip { }
	if s == "" {
		*a = []string{}
		return nil
	}
	parts := splitPGArray(s)
	*a = parts
	return nil
}

func splitPGArray(s string) []string {
	var out []string
	cur := ""
	for _, ch := range s {
		if ch == ',' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(ch)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// ─── Stub handlers ─────────────────────────────────────────────────────────────

func todo() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"code": "NOT_IMPLEMENTED", "message": "coming soon"})
	}
}

func handleChatCompletion(b *gateway.BrainClient) gin.HandlerFunc  { return todo() }
func handleChatStream(b *gateway.BrainClient, _ *streaming.Hub) gin.HandlerFunc {
	return todo()
}
func handleCreateSession(b *gateway.BrainClient) gin.HandlerFunc  { return todo() }
func handleGetSession(b *gateway.BrainClient) gin.HandlerFunc     { return todo() }
func handleGetMessages(b *gateway.BrainClient) gin.HandlerFunc    { return todo() }
func handleStoreMemory(b *gateway.BrainClient) gin.HandlerFunc    { return todo() }
func handleRecallMemory(b *gateway.BrainClient) gin.HandlerFunc   { return todo() }
func handleListMemory(b *gateway.BrainClient) gin.HandlerFunc     { return todo() }
func handleDeleteMemory(b *gateway.BrainClient) gin.HandlerFunc   { return todo() }
func handleIngestDocument(b *gateway.BrainClient) gin.HandlerFunc { return todo() }
func handleRAGQuery(b *gateway.BrainClient) gin.HandlerFunc       { return todo() }
func handleRAGQueryStream(b *gateway.BrainClient, _ *streaming.Hub) gin.HandlerFunc {
	return todo()
}
func handleDeleteDocument(b *gateway.BrainClient) gin.HandlerFunc { return todo() }
func handleListAgents(b *gateway.BrainClient) gin.HandlerFunc     { return todo() }
func handleRunAgent(b *gateway.BrainClient) gin.HandlerFunc       { return todo() }
func handleRunAgentStream(b *gateway.BrainClient, _ *streaming.Hub) gin.HandlerFunc {
	return todo()
}
func handleEnqueueTask(r *gateway.RunnerClient) gin.HandlerFunc  { return todo() }
func handleListTasks(r *gateway.RunnerClient) gin.HandlerFunc    { return todo() }
func handleGetTask(r *gateway.RunnerClient) gin.HandlerFunc      { return todo() }
func handleCancelTask(r *gateway.RunnerClient) gin.HandlerFunc   { return todo() }
func handleCreateWorkflow(r *gateway.RunnerClient) gin.HandlerFunc { return todo() }
func handleRunWorkflow(r *gateway.RunnerClient) gin.HandlerFunc  { return todo() }
func handleGetWorkflowRun(r *gateway.RunnerClient) gin.HandlerFunc { return todo() }
func handleTaskLogs(r *gateway.RunnerClient, _ *streaming.Hub) gin.HandlerFunc {
	return todo()
}

func prometheusHandler() gin.HandlerFunc {
	return func(c *gin.Context) { c.Status(http.StatusOK) }
}

