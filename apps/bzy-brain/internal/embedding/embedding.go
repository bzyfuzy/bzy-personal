// Package embedding provides text → vector embedding with batching and caching.
package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
)

// Service generates embeddings, with Redis caching to avoid redundant API calls.
type Service struct {
	provider providers.EmbeddingProvider
	cache    *redis.Client
	cacheTTL time.Duration
}

func NewService(reg providers.Registry, rdb *redis.Client) *Service {
	return &Service{
		provider: reg.Default(),
		cache:    rdb,
		cacheTTL: 24 * time.Hour,
	}
}

// Embed returns embeddings for a slice of texts, using cache for known texts.
func (s *Service) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	missing := make([]int, 0, len(texts))
	missingTexts := make([]string, 0, len(texts))

	for i, t := range texts {
		if v, err := s.getCache(ctx, t); err == nil {
			results[i] = v
		} else {
			missing = append(missing, i)
			missingTexts = append(missingTexts, t)
		}
	}

	if len(missingTexts) == 0 {
		return results, nil
	}

	vecs, err := s.provider.Embed(ctx, missingTexts)
	if err != nil {
		return nil, fmt.Errorf("embedding provider: %w", err)
	}

	for j, idx := range missing {
		results[idx] = vecs[j]
		_ = s.setCache(ctx, texts[idx], vecs[j])
	}
	return results, nil
}

// EmbedOne is a convenience wrapper for single texts.
func (s *Service) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	vecs, err := s.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

func (s *Service) Dimensions() int { return s.provider.Dimensions() }

func cacheKey(text string) string {
	h := sha256.Sum256([]byte(text))
	return "embed:" + hex.EncodeToString(h[:])
}

func (s *Service) getCache(ctx context.Context, text string) ([]float32, error) {
	raw, err := s.cache.Get(ctx, cacheKey(text)).Bytes()
	if err != nil {
		return nil, err
	}
	var v []float32
	return v, json.Unmarshal(raw, &v)
}

func (s *Service) setCache(ctx context.Context, text string, vec []float32) error {
	raw, err := json.Marshal(vec)
	if err != nil {
		return err
	}
	return s.cache.Set(ctx, cacheKey(text), raw, s.cacheTTL).Err()
}
