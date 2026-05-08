// Package scheduler provides cron-based workflow scheduling built on robfig/cron.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/queue"
)

// ScheduledJob defines a recurring task.
type ScheduledJob struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	CronExpr string          `json:"cron_expr"` // e.g. "0 9 * * 1-5"
	TaskType string          `json:"task_type"`
	Payload  []byte          `json:"payload"`
	Enabled  bool            `json:"enabled"`
	LastRun  *time.Time      `json:"last_run,omitempty"`
	NextRun  *time.Time      `json:"next_run,omitempty"`
	cronID   cron.EntryID
}

// CronScheduler manages scheduled jobs using cron expressions.
type CronScheduler struct {
	cron   *cron.Cron
	queue  queue.Queue
	jobs   map[string]*ScheduledJob
	logger *zap.Logger
}

func NewCronScheduler(q queue.Queue, logger *zap.Logger) *CronScheduler {
	c := cron.New(
		cron.WithSeconds(), // enable 6-field cron (with seconds)
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		),
	)
	return &CronScheduler{cron: c, queue: q, jobs: make(map[string]*ScheduledJob), logger: logger}
}

func (s *CronScheduler) Start() {
	s.cron.Start()
	s.logger.Info("cron scheduler started")
}

func (s *CronScheduler) Stop(ctx context.Context) error {
	s.cron.Stop()
	return nil
}

// AddJob registers a new cron job. Returns the job ID.
func (s *CronScheduler) AddJob(job *ScheduledJob) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	entryID, err := s.cron.AddFunc(job.CronExpr, func() {
		s.runJob(job)
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", job.CronExpr, err)
	}

	job.cronID = entryID
	s.jobs[job.ID] = job

	entry := s.cron.Entry(entryID)
	next := entry.Next
	job.NextRun = &next

	s.logger.Info("cron job registered",
		zap.String("id", job.ID),
		zap.String("name", job.Name),
		zap.String("expr", job.CronExpr),
		zap.Time("next_run", next),
	)
	return nil
}

// RemoveJob unregisters a cron job.
func (s *CronScheduler) RemoveJob(id string) error {
	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	s.cron.Remove(job.cronID)
	delete(s.jobs, id)
	return nil
}

// ListJobs returns all registered jobs.
func (s *CronScheduler) ListJobs() []*ScheduledJob {
	out := make([]*ScheduledJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		entry := s.cron.Entry(j.cronID)
		next := entry.Next
		j.NextRun = &next
		out = append(out, j)
	}
	return out
}

func (s *CronScheduler) runJob(job *ScheduledJob) {
	now := time.Now().UTC()
	job.LastRun = &now

	task := &queue.Task{
		ID:          uuid.New().String(),
		Type:        job.TaskType,
		Payload:     job.Payload,
		Priority:    queue.PriorityNormal,
		MaxAttempts: 3,
		Timeout:     30 * time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.queue.Enqueue(ctx, task); err != nil {
		s.logger.Error("failed to enqueue scheduled job",
			zap.String("job_id", job.ID),
			zap.Error(err),
		)
		return
	}

	s.logger.Info("scheduled task enqueued",
		zap.String("job_id", job.ID),
		zap.String("task_id", task.ID),
	)
}
