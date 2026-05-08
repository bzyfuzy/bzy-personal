// Package executor implements task execution backends: local process and Docker container.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ─── Result type ──────────────────────────────────────────────────────────────

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// ─── Executor interface ───────────────────────────────────────────────────────

// Executor runs commands in an isolated environment.
type Executor interface {
	// Run executes a command and returns its output.
	Run(ctx context.Context, req RunRequest) (*Result, error)
	// Type identifies the executor backend.
	Type() string
}

// RunRequest describes an execution request.
type RunRequest struct {
	Command []string
	WorkDir string
	Env     map[string]string
	Stdin   string
	Timeout time.Duration

	// Docker-specific
	Image  string
	Memory string
	CPU    string
}

// ─── Local executor ───────────────────────────────────────────────────────────

type LocalExecutor struct{}

func NewLocalExecutor() *LocalExecutor { return &LocalExecutor{} }

func (e *LocalExecutor) Type() string { return "local" }

func (e *LocalExecutor) Run(ctx context.Context, req RunRequest) (*Result, error) {
	if len(req.Command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, req.Command[0], req.Command[1:]...)
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)

	res := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: dur,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		}
		return res, fmt.Errorf("command failed: %w", err)
	}
	return res, nil
}

// ─── Docker executor ──────────────────────────────────────────────────────────

// DockerExecutor runs tasks in isolated Docker containers.
// Provides resource limits, network isolation, and clean environments.
type DockerExecutor struct {
	defaultImage  string
	defaultMemory string
	defaultCPU    string
	network       string
}

func NewDockerExecutor(defaultImage, memory, cpu, network string) *DockerExecutor {
	return &DockerExecutor{
		defaultImage:  defaultImage,
		defaultMemory: memory,
		defaultCPU:    cpu,
		network:       network,
	}
}

func (e *DockerExecutor) Type() string { return "docker" }

func (e *DockerExecutor) Run(ctx context.Context, req RunRequest) (*Result, error) {
	image := req.Image
	if image == "" {
		image = e.defaultImage
	}
	memory := req.Memory
	if memory == "" {
		memory = e.defaultMemory
	}

	args := []string{
		"run", "--rm",
		"--memory", memory,
		"--network", e.network,
		"--security-opt", "no-new-privileges",
	}
	for k, v := range req.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, image)
	args = append(args, req.Command...)

	dockerReq := RunRequest{
		Command: append([]string{"docker"}, args...),
		Timeout: req.Timeout,
		WorkDir: req.WorkDir,
	}

	return (&LocalExecutor{}).Run(ctx, dockerReq)
}
