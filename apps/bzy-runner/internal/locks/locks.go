// Package locks provides distributed locking using Redis.
// Uses SET NX PX (atomic lock acquisition) + Lua script for safe release.
package locks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrLockNotAcquired = errors.New("lock not acquired")

// Locker manages distributed locks.
type Locker interface {
	// Acquire attempts to acquire a lock. Returns ErrLockNotAcquired if unavailable.
	Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error)
	// TryAcquire keeps retrying until timeout.
	TryAcquire(ctx context.Context, key string, ttl, timeout time.Duration) (*Lock, error)
}

// Lock represents a held distributed lock.
type Lock struct {
	key    string
	token  string
	client *redis.Client
}

// Release releases the lock atomically (only if we still own it).
func (l *Lock) Release(ctx context.Context) error {
	// Lua script: only delete if the value matches our token (prevents releasing other holders)
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)
	return script.Run(ctx, l.client, []string{l.key}, l.token).Err()
}

// Extend refreshes the lock TTL.
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	return script.Run(ctx, l.client, []string{l.key}, l.token, ttl.Milliseconds()).Err()
}

// ─── Redis implementation ─────────────────────────────────────────────────────

type redisLocker struct {
	client *redis.Client
}

func NewRedisLocker(rdb *redis.Client) Locker {
	return &redisLocker{client: rdb}
}

func (r *redisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	token := uuid.New().String()
	lockKey := fmt.Sprintf("bzy:lock:%s", key)

	ok, err := r.client.SetNX(ctx, lockKey, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("lock acquire: %w", err)
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}
	return &Lock{key: lockKey, token: token, client: r.client}, nil
}

func (r *redisLocker) TryAcquire(ctx context.Context, key string, ttl, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)
	backoff := 50 * time.Millisecond

	for time.Now().Before(deadline) {
		lock, err := r.Acquire(ctx, key, ttl)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, ErrLockNotAcquired) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			if backoff < 500*time.Millisecond {
				backoff *= 2
			}
		}
	}
	return nil, fmt.Errorf("lock timeout after %s: %w", timeout, ErrLockNotAcquired)
}
