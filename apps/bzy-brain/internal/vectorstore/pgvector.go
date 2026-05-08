package vectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	pgvector "github.com/pgvector/pgvector-go"

	brainconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-brain/config"
)

// PgVector is the pgvector-backed Store implementation.
type PgVector struct {
	db *sql.DB
}

// NewPgVector opens a postgres connection and validates the schema.
func NewPgVector(cfg *brainconfig.Config) (*PgVector, error) {
	db, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("vectorstore: open db: %w", err)
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("vectorstore: ping: %w", err)
	}
	return &PgVector{db: db}, nil
}

func (s *PgVector) Upsert(ctx context.Context, v *Vector) error {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	meta, _ := json.Marshal(v.Metadata)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO vectors (id, collection, content, embedding, metadata)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE
		  SET content = EXCLUDED.content,
		      embedding = EXCLUDED.embedding,
		      metadata = EXCLUDED.metadata`,
		v.ID, v.Collection, v.Content, pgvector.NewVector(v.Embedding), meta,
	)
	return err
}

func (s *PgVector) UpsertBatch(ctx context.Context, vectors []*Vector) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO vectors (id, collection, content, embedding, metadata)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE
		  SET content = EXCLUDED.content,
		      embedding = EXCLUDED.embedding,
		      metadata = EXCLUDED.metadata`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, v := range vectors {
		if v.ID == "" {
			v.ID = uuid.NewString()
		}
		meta, _ := json.Marshal(v.Metadata)
		if _, err := stmt.ExecContext(ctx, v.ID, v.Collection, v.Content, pgvector.NewVector(v.Embedding), meta); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PgVector) Search(ctx context.Context, q SearchQuery) ([]*Vector, error) {
	args := []any{q.Collection, pgvector.NewVector(q.Embedding), q.TopK}
	where := "collection = $1"
	if len(q.Filters) > 0 {
		i := 4
		for k, v := range q.Filters {
			where += fmt.Sprintf(" AND metadata->>'%s' = $%d", strings.ReplaceAll(k, "'", ""), i)
			args = append(args, v)
			i++
		}
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, collection, content, embedding, metadata, created_at,
		       1 - (embedding <=> $2) AS score
		FROM   vectors
		WHERE  %s
		  AND  1 - (embedding <=> $2) >= %g
		ORDER  BY embedding <=> $2
		LIMIT  $3`, where, q.MinScore), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVectors(rows)
}

func (s *PgVector) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM vectors WHERE id = $1`, id)
	return err
}

func (s *PgVector) DeleteCollection(ctx context.Context, collection string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM vectors WHERE collection = $1`, collection)
	return err
}

func (s *PgVector) Get(ctx context.Context, id string) (*Vector, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, collection, content, embedding, metadata, created_at, 0 AS score
		 FROM vectors WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vs, err := scanVectors(rows)
	if err != nil {
		return nil, err
	}
	if len(vs) == 0 {
		return nil, fmt.Errorf("vector %s: not found", id)
	}
	return vs[0], nil
}

func scanVectors(rows *sql.Rows) ([]*Vector, error) {
	var out []*Vector
	for rows.Next() {
		var v Vector
		var emb pgvector.Vector
		var meta []byte
		if err := rows.Scan(&v.ID, &v.Collection, &v.Content, &emb, &meta, &v.CreatedAt, &v.Score); err != nil {
			return nil, err
		}
		v.Embedding = emb.Slice()
		if meta != nil {
			_ = json.Unmarshal(meta, &v.Metadata)
		}
		out = append(out, &v)
	}
	return out, rows.Err()
}
