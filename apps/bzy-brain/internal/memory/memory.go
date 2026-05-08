// Package memory manages conversation history, semantic recall, and long-term storage.
package memory

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ─── Domain model ─────────────────────────────────────────────────────────────

type MemoryType string

const (
	TypeEpisodic  MemoryType = "episodic"   // conversation turns
	TypeSemantic  MemoryType = "semantic"   // facts and knowledge
	TypeProcedural MemoryType = "procedural" // how-to knowledge
	TypeWorking   MemoryType = "working"    // current session context
)

// Memory is a single stored memory unit.
type Memory struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id,omitempty"`
	UserID     string         `json:"user_id"`
	Type       MemoryType     `json:"type"`
	Content    string         `json:"content"`
	Summary    string         `json:"summary,omitempty"`
	Importance float32        `json:"importance"` // 0–1 computed score
	Metadata   map[string]any `json:"metadata,omitempty"`
	Embedding  []float32      `json:"-"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
}

// Session tracks an active conversation.
type Session struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title,omitempty"`
	Messages  []Message  `json:"messages"`
	Summary   string     `json:"summary,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// RecallResult pairs a memory with its similarity score.
type RecallResult struct {
	Memory *Memory `json:"memory"`
	Score  float32 `json:"score"`
}

// ─── Service interface ─────────────────────────────────────────────────────────

type Service interface {
	// Store saves a new memory and generates its embedding.
	Store(ctx context.Context, m *Memory) error
	// Recall performs semantic search over memories for a user.
	Recall(ctx context.Context, userID, query string, topK int) ([]RecallResult, error)
	// Forget deletes a specific memory.
	Forget(ctx context.Context, id string) error
	// GetSession retrieves a session with its message history.
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	// AddMessage appends a message to a session and triggers memory consolidation.
	AddMessage(ctx context.Context, sessionID string, msg Message) error
	// SummarizeSession produces a rolling summary when message count exceeds threshold.
	SummarizeSession(ctx context.Context, sessionID string) error
	// BuildContext assembles a ranked context window for an AI call.
	BuildContext(ctx context.Context, userID, sessionID, query string, maxTokens int) ([]Message, error)
}

// ─── Repository interface ─────────────────────────────────────────────────────

type Repository interface {
	Save(ctx context.Context, m *Memory) error
	FindByID(ctx context.Context, id string) (*Memory, error)
	FindByUser(ctx context.Context, userID string, limit int) ([]*Memory, error)
	Delete(ctx context.Context, id string) error
	SaveSession(ctx context.Context, s *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	AppendMessage(ctx context.Context, sessionID string, msg Message) error
	UpdateSummary(ctx context.Context, sessionID, summary string) error
}

// ─── Concrete service ─────────────────────────────────────────────────────────

type memoryService struct {
	repo      Repository
	embedder  interface {
		EmbedOne(ctx context.Context, text string) ([]float32, error)
	}
	vecStore  interface {
		Search(ctx context.Context, query interface{}) (interface{}, error)
	}
}

func NewService(repo Repository) Service {
	return &memoryService{repo: repo}
}

func (s *memoryService) Store(ctx context.Context, m *Memory) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = m.CreatedAt
	return s.repo.Save(ctx, m)
}

func (s *memoryService) Recall(ctx context.Context, userID, query string, topK int) ([]RecallResult, error) {
	// Stub — real impl calls embedding + vectorstore.Search
	return nil, nil
}

func (s *memoryService) Forget(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *memoryService) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return s.repo.GetSession(ctx, sessionID)
}

func (s *memoryService) AddMessage(ctx context.Context, sessionID string, msg Message) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.CreatedAt = time.Now().UTC()
	return s.repo.AppendMessage(ctx, sessionID, msg)
}

func (s *memoryService) SummarizeSession(_ context.Context, _ string) error {
	// Triggered when len(messages) > config.SummaryThreshold
	// Calls reasoning engine to produce rolling summary
	return nil
}

func (s *memoryService) BuildContext(ctx context.Context, userID, sessionID, query string, maxTokens int) ([]Message, error) {
	sess, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	// Simple recency window — production impl applies token counting + semantic ranking
	msgs := sess.Messages
	if len(msgs) > 20 {
		msgs = msgs[len(msgs)-20:]
	}
	return msgs, nil
}
