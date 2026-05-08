// Package worker implements the worker pool that consumes and executes tasks.
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/executor"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/logstream"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/queue"
)

// Handler processes a specific task type.
type Handler interface {
	// Type returns the task type this handler processes.
	Type() string
	// Handle executes the task and returns a result or error.
	Handle(ctx context.Context, task *queue.Task, log logstream.Writer) error
}

// Pool manages a fixed set of goroutine workers.
type Pool struct {
	queue       queue.Queue
	exec        executor.Executor
	logWriter   logstream.Writer
	handlers    map[string]Handler
	concurrency int
	logger      *zap.Logger
	wg          sync.WaitGroup
	quit        chan struct{}
}

func NewPool(q queue.Queue, exec executor.Executor, ls logstream.Writer, concurrency int, logger *zap.Logger) *Pool {
	return &Pool{
		queue:       q,
		exec:        exec,
		logWriter:   ls,
		handlers:    make(map[string]Handler),
		concurrency: concurrency,
		logger:      logger,
		quit:        make(chan struct{}),
	}
}

func (p *Pool) Register(h Handler) {
	p.handlers[h.Type()] = h
}

// Start launches the worker goroutines. Blocks until Shutdown is called.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}
	p.logger.Info("worker pool started", zap.Int("concurrency", p.concurrency))
	p.wg.Wait()
}

func (p *Pool) runWorker(ctx context.Context, id int) {
	defer p.wg.Done()
	logger := p.logger.With(zap.Int("worker_id", id))

	for {
		select {
		case <-p.quit:
			logger.Debug("worker shutting down")
			return
		case <-ctx.Done():
			return
		default:
		}

		task, err := p.queue.Dequeue(ctx)
		if err != nil {
			logger.Error("dequeue error", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		if task == nil {
			continue // poll timeout, no task
		}

		p.processTask(ctx, task, logger)
	}
}

func (p *Pool) processTask(ctx context.Context, task *queue.Task, logger *zap.Logger) {
	start := time.Now()
	logger = logger.With(zap.String("task_id", task.ID), zap.String("task_type", task.Type))
	logger.Info("processing task")

	timeout := task.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	h, ok := p.handlers[task.Type]
	if !ok {
		logger.Warn("no handler for task type")
		_ = p.queue.Nack(ctx, task.ID, fmt.Sprintf("no handler for type: %s", task.Type))
		return
	}

	err := h.Handle(tctx, task, p.logWriter)
	elapsed := time.Since(start)

	if err != nil {
		logger.Error("task failed", zap.Error(err), zap.Duration("elapsed", elapsed))
		_ = p.queue.Nack(ctx, task.ID, err.Error())
		return
	}

	logger.Info("task completed", zap.Duration("elapsed", elapsed))
	_ = p.queue.Ack(ctx, task.ID, nil)
}

// Shutdown signals workers to stop and waits for in-flight tasks to complete.
func (p *Pool) Shutdown(ctx context.Context) error {
	close(p.quit)
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		p.logger.Info("worker pool shutdown complete")
		return nil
	case <-ctx.Done():
		p.logger.Warn("worker pool shutdown timeout — some tasks may be incomplete")
		return ctx.Err()
	}
}
