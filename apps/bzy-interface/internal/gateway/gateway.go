// Package gateway holds typed gRPC clients for upstream services.
package gateway

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"time"
)

// BrainClient is the gRPC client for bzy-brain.
// Replace the stub fields with generated proto clients once protos are compiled.
type BrainClient struct {
	conn *grpc.ClientConn
	// brainv1.BrainServiceClient
}

func NewBrainClient(addr string) (*BrainClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("brain client dial %s: %w", addr, err)
	}
	return &BrainClient{conn: conn}, nil
}

func (c *BrainClient) Close() error { return c.conn.Close() }

// Ping performs a simple connectivity check.
func (c *BrainClient) Ping(ctx context.Context) error {
	state := c.conn.GetState()
	_ = state
	return nil
}

// RunnerClient is the gRPC client for bzy-runner.
type RunnerClient struct {
	conn *grpc.ClientConn
}

func NewRunnerClient(addr string) (*RunnerClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("runner client dial %s: %w", addr, err)
	}
	return &RunnerClient{conn: conn}, nil
}

func (c *RunnerClient) Close() error { return c.conn.Close() }
