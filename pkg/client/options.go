package client

import (
	"net/http"
	"time"
)

// Config holds all client configuration.
type Config struct {
	BaseURL    string
	APIKey     string        // static API key (alternative to JWT)
	Timeout    time.Duration
	MaxRetries int
	RetryWait  time.Duration
	HTTPClient *http.Client
	UserAgent  string
}

// Option configures the client.
type Option func(*Config)

func WithAPIKey(key string) Option {
	return func(c *Config) { c.APIKey = key }
}

func WithTimeout(d time.Duration) Option {
	return func(c *Config) { c.Timeout = d }
}

func WithMaxRetries(n int) Option {
	return func(c *Config) { c.MaxRetries = n }
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Config) { c.HTTPClient = hc }
}

func WithUserAgent(ua string) Option {
	return func(c *Config) { c.UserAgent = ua }
}

func defaultConfig(baseURL string) Config {
	return Config{
		BaseURL:    baseURL,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryWait:  500 * time.Millisecond,
		UserAgent:  "bzy-client/1.0",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}
