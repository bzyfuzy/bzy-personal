package config

import (
	"github.com/bzyfuzy/bzy-personal/pkg/config"
)

type Config struct {
	config.Base `mapstructure:",squash"`

	GRPC     GRPC     `mapstructure:"grpc"`
	Database config.Database `mapstructure:"database"`

	AI       AI       `mapstructure:"ai"`
	Vector   Vector   `mapstructure:"vector"`
	Memory   Memory   `mapstructure:"memory"`
}

type GRPC struct {
	Addr string `mapstructure:"addr"`
}

type AI struct {
	DefaultProvider string `mapstructure:"default_provider"` // openai | ollama | anthropic
	OpenAI          OpenAI `mapstructure:"openai"`
	Ollama          Ollama `mapstructure:"ollama"`
}

type OpenAI struct {
	APIKey         string  `mapstructure:"api_key"`
	BaseURL        string  `mapstructure:"base_url"`
	DefaultModel   string  `mapstructure:"default_model"`
	EmbeddingModel string  `mapstructure:"embedding_model"`
	MaxTokens      int     `mapstructure:"max_tokens"`
	Temperature    float64 `mapstructure:"temperature"`
}

type Ollama struct {
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type Vector struct {
	Dimensions    int    `mapstructure:"dimensions"`
	DistanceFunc  string `mapstructure:"distance_func"` // cosine | l2 | ip
	IndexType     string `mapstructure:"index_type"`    // ivfflat | hnsw
}

type Memory struct {
	MaxContextTokens  int    `mapstructure:"max_context_tokens"`
	RecencyWindowDays int    `mapstructure:"recency_window_days"`
	SummaryThreshold  int    `mapstructure:"summary_threshold"` // messages before summarising
}

func Load() (*Config, error) {
	cfg := &Config{
		Base: config.Base{
			ServiceName: "bzy-brain",
			LogLevel:    "info",
			LogFormat:   "json",
			Env:         "development",
		},
		GRPC: GRPC{Addr: ":50051"},
		AI: AI{
			DefaultProvider: "openai",
			OpenAI: OpenAI{
				DefaultModel:   "gpt-4o",
				EmbeddingModel: "text-embedding-3-small",
				MaxTokens:      4096,
				Temperature:    0.7,
			},
		},
		Vector: Vector{
			Dimensions:   1536,
			DistanceFunc: "cosine",
			IndexType:    "hnsw",
		},
		Memory: Memory{
			MaxContextTokens:  8000,
			RecencyWindowDays: 30,
			SummaryThreshold:  20,
		},
	}
	return cfg, config.Load("BRAIN", cfg)
}
