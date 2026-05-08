package client

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// do executes fn with exponential backoff + jitter on retryable errors.
func (c *Client) do(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		if attempt == c.cfg.MaxRetries {
			break
		}

		wait := backoff(c.cfg.RetryWait, attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return err
}

// backoff returns exponential wait with ±25% jitter.
func backoff(base time.Duration, attempt int) time.Duration {
	exp := time.Duration(math.Pow(2, float64(attempt))) * base
	if exp > 30*time.Second {
		exp = 30 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(exp) / 4))
	return exp + jitter
}
