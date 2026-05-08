// Package heartbeat manages node registration and liveness within the cluster.
package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NodeInfo describes a worker node.
type NodeInfo struct {
	ID           string    `json:"id"`
	Hostname     string    `json:"hostname"`
	Addr         string    `json:"addr,omitempty"`
	Concurrency  int       `json:"concurrency"`
	ActiveTasks  int       `json:"active_tasks"`
	QueuedTasks  int       `json:"queued_tasks"`
	Version      string    `json:"version"`
	StartedAt    time.Time `json:"started_at"`
	LastSeen     time.Time `json:"last_seen"`
}

// Service manages heartbeats and node registration.
type Service struct {
	client   *redis.Client
	node     NodeInfo
	interval time.Duration
	ttl      time.Duration
	logger   *zap.Logger
	quit     chan struct{}
}

func NewService(rdb *redis.Client, nodeID string, concurrency int, interval, ttl time.Duration, logger *zap.Logger) *Service {
	hostname, _ := os.Hostname()
	return &Service{
		client:   rdb,
		interval: interval,
		ttl:      ttl,
		logger:   logger,
		quit:     make(chan struct{}),
		node: NodeInfo{
			ID:          nodeID,
			Hostname:    hostname,
			Concurrency: concurrency,
			Version:     "dev",
			StartedAt:   time.Now().UTC(),
		},
	}
}

// Run sends heartbeats on the configured interval until Stop() is called.
func (s *Service) Run(ctx context.Context) {
	if err := s.register(ctx); err != nil {
		s.logger.Error("failed to register node", zap.Error(err))
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.beat(ctx); err != nil {
				s.logger.Warn("heartbeat failed", zap.Error(err))
			}
		case <-s.quit:
			_ = s.deregister(ctx)
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) Stop() error {
	close(s.quit)
	return nil
}

func (s *Service) register(ctx context.Context) error {
	s.logger.Info("registering node", zap.String("node_id", s.node.ID))
	return s.beat(ctx)
}

func (s *Service) beat(ctx context.Context) error {
	s.node.LastSeen = time.Now().UTC()
	raw, err := json.Marshal(s.node)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("bzy:nodes:%s", s.node.ID)
	return s.client.Set(ctx, key, raw, s.ttl).Err()
}

func (s *Service) deregister(ctx context.Context) error {
	s.logger.Info("deregistering node", zap.String("node_id", s.node.ID))
	return s.client.Del(ctx, fmt.Sprintf("bzy:nodes:%s", s.node.ID)).Err()
}

// UpdateMetrics updates live metrics on the node info (call from worker pool).
func (s *Service) UpdateMetrics(activeTasks, queuedTasks int) {
	s.node.ActiveTasks = activeTasks
	s.node.QueuedTasks = queuedTasks
}
