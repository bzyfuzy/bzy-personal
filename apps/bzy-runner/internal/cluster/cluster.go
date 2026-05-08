// Package cluster manages multi-node worker registration and discovery.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/heartbeat"
)

// Registry discovers and tracks all active worker nodes.
type Registry struct {
	client *redis.Client
	logger *zap.Logger
	nodeID string
}

func NewRegistry(rdb *redis.Client, nodeID string, logger *zap.Logger) *Registry {
	return &Registry{client: rdb, logger: logger, nodeID: nodeID}
}

// ListNodes returns all currently alive worker nodes.
func (r *Registry) ListNodes(ctx context.Context) ([]*heartbeat.NodeInfo, error) {
	pattern := "bzy:nodes:*"
	var cursor uint64
	var keys []string

	for {
		batch, next, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("cluster scan: %w", err)
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}

	nodes := make([]*heartbeat.NodeInfo, 0, len(keys))
	for _, key := range keys {
		raw, err := r.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}
		var n heartbeat.NodeInfo
		if err := json.Unmarshal(raw, &n); err == nil {
			nodes = append(nodes, &n)
		}
	}
	return nodes, nil
}

// GetNode returns info for a specific node.
func (r *Registry) GetNode(ctx context.Context, nodeID string) (*heartbeat.NodeInfo, error) {
	raw, err := r.client.Get(ctx, fmt.Sprintf("bzy:nodes:%s", nodeID)).Bytes()
	if err != nil {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	var n heartbeat.NodeInfo
	return &n, json.Unmarshal(raw, &n)
}

// HealthyNodes returns nodes that have sent a heartbeat within the last ttl.
func (r *Registry) HealthyNodes(ctx context.Context, ttl time.Duration) ([]*heartbeat.NodeInfo, error) {
	all, err := r.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-ttl)
	var healthy []*heartbeat.NodeInfo
	for _, n := range all {
		if n.LastSeen.After(cutoff) {
			healthy = append(healthy, n)
		}
	}
	return healthy, nil
}

// TotalCapacity returns total available worker slots across the cluster.
func (r *Registry) TotalCapacity(ctx context.Context) (int, error) {
	nodes, err := r.HealthyNodes(ctx, 30*time.Second)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, n := range nodes {
		total += n.Concurrency - n.ActiveTasks
	}
	return total, nil
}

// ExtractNodeID returns the nodeID portion from a Redis key "bzy:nodes:<nodeID>".
func ExtractNodeID(key string) string {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return key
}
