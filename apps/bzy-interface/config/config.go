package config

import (
	"time"

	"github.com/bzyfuzy/bzy-personal/pkg/config"
)

type Config struct {
	config.Base `mapstructure:",squash"`

	HTTP      HTTP            `mapstructure:"http"`
	Auth      Auth            `mapstructure:"auth"`
	Gateway   Gateway         `mapstructure:"gateway"`
	RateLimit RateLimit       `mapstructure:"rate_limit"`
	Database  config.Database `mapstructure:"database"`
}

type HTTP struct {
	Addr            string        `mapstructure:"addr"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	CORSOrigins     []string      `mapstructure:"cors_origins"`
}

type Auth struct {
	JWTSecret      string        `mapstructure:"jwt_secret"`
	JWTExpiry      time.Duration `mapstructure:"jwt_expiry"`
	RefreshExpiry  time.Duration `mapstructure:"refresh_expiry"`
	APIKeyHeader   string        `mapstructure:"api_key_header"`
}

type Gateway struct {
	BrainAddr  string `mapstructure:"brain_addr"`
	RunnerAddr string `mapstructure:"runner_addr"`
}

type RateLimit struct {
	RequestsPerMin int `mapstructure:"requests_per_min"`
	BurstSize      int `mapstructure:"burst_size"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Base: config.Base{
			ServiceName: "bzy-interface",
			LogLevel:    "info",
			LogFormat:   "json",
		},
		HTTP: HTTP{
			Addr:            ":8080",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    60 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Auth: Auth{
			JWTExpiry:     24 * time.Hour,
			RefreshExpiry: 7 * 24 * time.Hour,
			APIKeyHeader:  "X-API-Key",
		},
		Gateway: Gateway{
			BrainAddr:  "localhost:50051",
			RunnerAddr: "localhost:50052",
		},
		RateLimit: RateLimit{
			RequestsPerMin: 120,
			BurstSize:      20,
		},
		Database: config.Database{
			DSN:             "postgres://bzy:bzy_dev@localhost:5432/bzy_brain?sslmode=disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
	}
	return cfg, config.Load("INTERFACE", cfg)
}
