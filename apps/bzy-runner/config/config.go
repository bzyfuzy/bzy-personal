package config

import (
	"github.com/bzyfuzy/bzy-personal/pkg/config"
)

type Config struct {
	config.Base `mapstructure:",squash"`

	Database config.Database `mapstructure:"database"`
	Worker   Worker          `mapstructure:"worker"`
	Executor Executor        `mapstructure:"executor"`
	Cluster  Cluster         `mapstructure:"cluster"`
}

type Worker struct {
	Concurrency     int    `mapstructure:"concurrency"`      // parallel workers per node
	QueueName       string `mapstructure:"queue_name"`
	DLQName         string `mapstructure:"dlq_name"`
	MaxRetries      int    `mapstructure:"max_retries"`
	RetryBackoffSec int    `mapstructure:"retry_backoff_sec"`
	PollIntervalMs  int    `mapstructure:"poll_interval_ms"`
	TaskTimeoutSec  int    `mapstructure:"task_timeout_sec"`
}

type Executor struct {
	Type           string `mapstructure:"type"`            // local | docker
	DockerNetwork  string `mapstructure:"docker_network"`
	DockerMemory   string `mapstructure:"docker_memory"`   // e.g. "512m"
	DockerCPU      string `mapstructure:"docker_cpu"`      // e.g. "0.5"
	SandboxEnabled bool   `mapstructure:"sandbox_enabled"`
}

type Cluster struct {
	NodeID             string `mapstructure:"node_id"`
	HeartbeatIntervalSec int  `mapstructure:"heartbeat_interval_sec"`
	NodeTTLSec         int    `mapstructure:"node_ttl_sec"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Base: config.Base{
			ServiceName: "bzy-runner",
			LogLevel:    "info",
			LogFormat:   "json",
		},
		Worker: Worker{
			Concurrency:     4,
			QueueName:       "bzy:tasks",
			DLQName:         "bzy:tasks:dlq",
			MaxRetries:      3,
			RetryBackoffSec: 5,
			PollIntervalMs:  500,
			TaskTimeoutSec:  300,
		},
		Executor: Executor{
			Type:           "local",
			DockerMemory:   "512m",
			SandboxEnabled: false,
		},
		Cluster: Cluster{
			HeartbeatIntervalSec: 10,
			NodeTTLSec:           30,
		},
	}
	return cfg, config.Load("RUNNER", cfg)
}
