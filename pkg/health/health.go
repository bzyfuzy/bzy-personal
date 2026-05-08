// Package health provides a composable health-check system for all services.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Status string

const (
	StatusUp      Status = "up"
	StatusDown    Status = "down"
	StatusDegraded Status = "degraded"
)

// Check is a named liveness or readiness probe.
type Check struct {
	Name    string
	Timeout time.Duration
	Fn      func(ctx context.Context) error
}

// Result is the output of a single check.
type Result struct {
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Error   string        `json:"error,omitempty"`
	Latency time.Duration `json:"latency_ms"`
}

// Report aggregates results across all checks.
type Report struct {
	Status  Status    `json:"status"`
	Checks  []Result  `json:"checks"`
	Version string    `json:"version,omitempty"`
	At      time.Time `json:"at"`
}

// Checker runs registered health checks.
type Checker struct {
	mu      sync.RWMutex
	checks  []Check
	version string
}

func New(version string) *Checker {
	return &Checker{version: version}
}

func (c *Checker) Register(check Check) {
	if check.Timeout == 0 {
		check.Timeout = 5 * time.Second
	}
	c.mu.Lock()
	c.checks = append(c.checks, check)
	c.mu.Unlock()
}

func (c *Checker) Run(ctx context.Context) Report {
	c.mu.RLock()
	checks := make([]Check, len(c.checks))
	copy(checks, c.checks)
	c.mu.RUnlock()

	results := make([]Result, len(checks))
	var wg sync.WaitGroup
	for i, ch := range checks {
		wg.Add(1)
		go func(i int, ch Check) {
			defer wg.Done()
			tctx, cancel := context.WithTimeout(ctx, ch.Timeout)
			defer cancel()
			start := time.Now()
			err := ch.Fn(tctx)
			lat := time.Since(start)
			r := Result{Name: ch.Name, Latency: lat}
			if err != nil {
				r.Status = StatusDown
				r.Error = err.Error()
			} else {
				r.Status = StatusUp
			}
			results[i] = r
		}(i, ch)
	}
	wg.Wait()

	overall := StatusUp
	for _, r := range results {
		if r.Status == StatusDown {
			overall = StatusDown
			break
		}
	}
	return Report{Status: overall, Checks: results, Version: c.version, At: time.Now().UTC()}
}

// LivenessHandler returns 200 if the process is alive (always true once running).
func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "up"})
	}
}

// ReadinessHandler runs all checks and returns 200 only if all pass.
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := c.Run(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if report.Status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(report)
	}
}
