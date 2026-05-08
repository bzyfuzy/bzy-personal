package memory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) Save(ctx context.Context, m *Memory) error {
	meta, _ := json.Marshal(m.Metadata)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO memories (id, session_id, user_id, type, content, summary, importance, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE
		SET content=EXCLUDED.content, summary=EXCLUDED.summary,
		    importance=EXCLUDED.importance, metadata=EXCLUDED.metadata, updated_at=EXCLUDED.updated_at`,
		m.ID, m.SessionID, m.UserID, string(m.Type), m.Content,
		m.Summary, m.Importance, meta, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (r *pgRepository) FindByID(ctx context.Context, id string) (*Memory, error) {
	m := &Memory{}
	var meta json.RawMessage
	err := r.pool.QueryRow(ctx,
		`SELECT id, session_id, user_id, type, content, summary, importance, metadata, created_at, updated_at
		 FROM memories WHERE id = $1`, id,
	).Scan(&m.ID, &m.SessionID, &m.UserID, &m.Type, &m.Content, &m.Summary,
		&m.Importance, &meta, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("memory not found: %w", err)
	}
	_ = json.Unmarshal(meta, &m.Metadata)
	return m, nil
}

func (r *pgRepository) FindByUser(ctx context.Context, userID string, limit int) ([]*Memory, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, session_id, user_id, type, content, summary, importance, metadata, created_at, updated_at
		 FROM memories WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Memory
	for rows.Next() {
		m := &Memory{}
		var meta json.RawMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.UserID, &m.Type, &m.Content, &m.Summary,
			&m.Importance, &meta, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(meta, &m.Metadata)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *pgRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM memories WHERE id = $1`, id)
	return err
}

func (r *pgRepository) SaveSession(ctx context.Context, s *Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, title, summary, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE
		SET title=EXCLUDED.title, summary=EXCLUDED.summary, updated_at=EXCLUDED.updated_at`,
		s.ID, s.UserID, s.Title, s.Summary, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *pgRepository) GetSession(ctx context.Context, id string) (*Session, error) {
	s := &Session{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, title, summary, created_at, updated_at FROM sessions WHERE id = $1`, id,
	).Scan(&s.ID, &s.UserID, &s.Title, &s.Summary, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, role, content, created_at FROM messages WHERE session_id=$1 ORDER BY created_at ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		s.Messages = append(s.Messages, m)
	}
	return s, rows.Err()
}

func (r *pgRepository) AppendMessage(ctx context.Context, sessionID string, msg Message) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO messages (id, session_id, role, content, created_at) VALUES ($1,$2,$3,$4,$5)`,
		msg.ID, sessionID, msg.Role, msg.Content, msg.CreatedAt)
	return err
}

func (r *pgRepository) UpdateSummary(ctx context.Context, sessionID, summary string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET summary=$1 WHERE id=$2`, summary, sessionID)
	return err
}
