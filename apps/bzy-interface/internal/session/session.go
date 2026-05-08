// Package session manages user session tokens in Redis.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Data holds session state.
type Data struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// Manager stores and retrieves sessions in Redis.
type Manager struct {
	client *redis.Client
	ttl    time.Duration
}

func NewManager(rdb *redis.Client, ttl time.Duration) *Manager {
	return &Manager{client: rdb, ttl: ttl}
}

func sessionKey(token string) string { return fmt.Sprintf("bzy:session:%s", token) }

// Create persists a session and returns the opaque session token.
func (m *Manager) Create(ctx context.Context, data *Data) (string, error) {
	token := uuid.New().String()
	data.CreatedAt = time.Now().UTC()
	data.LastSeen = data.CreatedAt

	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return token, m.client.Set(ctx, sessionKey(token), raw, m.ttl).Err()
}

// Get retrieves and refreshes a session.
func (m *Manager) Get(ctx context.Context, token string) (*Data, error) {
	raw, err := m.client.Get(ctx, sessionKey(token)).Bytes()
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	// Touch TTL
	_ = m.client.Expire(ctx, sessionKey(token), m.ttl)
	return &data, nil
}

// Delete invalidates a session (logout).
func (m *Manager) Delete(ctx context.Context, token string) error {
	return m.client.Del(ctx, sessionKey(token)).Err()
}
