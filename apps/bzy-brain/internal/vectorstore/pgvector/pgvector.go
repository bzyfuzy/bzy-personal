// Package pgvector implements vectorstore.Store using PostgreSQL + pgvector.
package pgvector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvec "github.com/pgvector/pgvector-go"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/vectorstore"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// NewPgVector is the fx-compatible constructor.
func NewPgVector(pool *pgxpool.Pool) vectorstore.Store {
	return NewStore(pool)
}

func (s *Store) Upsert(ctx context.Context, v *vectorstore.Vector) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	meta, err := json.Marshal(v.Metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO vectors (id, collection, content, embedding, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET content = EXCLUDED.content,
		    embedding = EXCLUDED.embedding,
		    metadata = EXCLUDED.metadata`,
		v.ID, v.Collection, v.Content,
		pgvec.NewVector(v.Embedding), meta, v.CreatedAt,
	)
	return err
}

func (s *Store) UpsertBatch(ctx context.Context, vectors []*vectorstore.Vector) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, v := range vectors {
		if err := s.upsertTx(ctx, tx, v); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) upsertTx(ctx context.Context, tx pgx.Tx, v *vectorstore.Vector) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	meta, _ := json.Marshal(v.Metadata)
	_, err := tx.Exec(ctx, `
		INSERT INTO vectors (id, collection, content, embedding, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE
		SET content = EXCLUDED.content, embedding = EXCLUDED.embedding, metadata = EXCLUDED.metadata`,
		v.ID, v.Collection, v.Content, pgvec.NewVector(v.Embedding), meta,
	)
	return err
}

func (s *Store) Search(ctx context.Context, q vectorstore.SearchQuery) ([]*vectorstore.Vector, error) {
	if q.TopK == 0 {
		q.TopK = 10
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, collection, content, embedding, metadata, created_at,
		       1 - (embedding <=> $1) AS score
		FROM vectors
		WHERE collection = $2
		  AND 1 - (embedding <=> $1) >= $3
		ORDER BY embedding <=> $1
		LIMIT $4`,
		pgvec.NewVector(q.Embedding), q.Collection, q.MinScore, q.TopK,
	)
	if err != nil {
		return nil, fmt.Errorf("pgvector search: %w", err)
	}
	defer rows.Close()

	var results []*vectorstore.Vector
	for rows.Next() {
		v := &vectorstore.Vector{}
		var emb pgvec.Vector
		var meta json.RawMessage
		if err := rows.Scan(&v.ID, &v.Collection, &v.Content, &emb, &meta, &v.CreatedAt, &v.Score); err != nil {
			return nil, err
		}
		v.Embedding = emb.Slice()
		_ = json.Unmarshal(meta, &v.Metadata)
		results = append(results, v)
	}
	return results, rows.Err()
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM vectors WHERE id = $1`, id)
	return err
}

func (s *Store) DeleteCollection(ctx context.Context, collection string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM vectors WHERE collection = $1`, collection)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*vectorstore.Vector, error) {
	v := &vectorstore.Vector{}
	var emb pgvec.Vector
	var meta json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT id, collection, content, embedding, metadata, created_at FROM vectors WHERE id = $1`, id,
	).Scan(&v.ID, &v.Collection, &v.Content, &emb, &meta, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	v.Embedding = emb.Slice()
	_ = json.Unmarshal(meta, &v.Metadata)
	return v, nil
}
