package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Base holds fields shared by every service.
type Base struct {
	Env        string `mapstructure:"env"`         // development | staging | production
	LogLevel   string `mapstructure:"log_level"`   // debug | info | warn | error
	LogFormat  string `mapstructure:"log_format"`  // json | console
	ServiceName string `mapstructure:"service_name"`

	Telemetry Telemetry `mapstructure:"telemetry"`
	Redis     Redis     `mapstructure:"redis"`
	NATS      NATS      `mapstructure:"nats"`
}

type Telemetry struct {
	Enabled      bool   `mapstructure:"enabled"`
	OTLPEndpoint string `mapstructure:"otlp_endpoint"`
	SampleRate   float64 `mapstructure:"sample_rate"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type NATS struct {
	URL            string        `mapstructure:"url"`
	MaxReconnect   int           `mapstructure:"max_reconnect"`
	ReconnectWait  time.Duration `mapstructure:"reconnect_wait"`
}

type Database struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	MigrationsPath  string        `mapstructure:"migrations_path"`
}

func (b *Base) IsProd() bool { return b.Env == "production" }
func (b *Base) IsDev() bool  { return b.Env == "development" }

// Load reads config from environment variables and optional file.
// Prefix is the service-specific env prefix (e.g. "BRAIN", "RUNNER", "INTERFACE").
func Load[T any](prefix string, cfg *T) error {
	v := viper.New()

	v.SetEnvPrefix(prefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("env", "development")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")
	v.SetDefault("telemetry.enabled", false)
	v.SetDefault("telemetry.sample_rate", 0.1)
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.max_reconnect", 10)
	v.SetDefault("nats.reconnect_wait", "2s")

	// Optional config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/bzy/")
	_ = v.ReadInConfig() // not required

	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("config unmarshal: %w", err)
	}
	return nil
}
