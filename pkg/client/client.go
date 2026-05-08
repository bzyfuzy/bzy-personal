// Package client provides a typed Go client for the bzy-personal API.
//
// Usage:
//
//	c := client.New("http://localhost:8080", client.WithAPIKey("sk-..."))
//
//	// or with JWT login
//	c := client.New("http://localhost:8080")
//	if err := c.Login(ctx, "user@example.com", "password"); err != nil { ... }
//
//	resp, err := c.Chat.Complete(ctx, client.ChatRequest{...})
//	stream, err := c.Chat.Stream(ctx, client.ChatRequest{...})
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Client is the top-level API client.
type Client struct {
	cfg Config
	mu  sync.RWMutex

	// auth state
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time

	// sub-clients (namespaced by resource)
	Auth      *AuthClient
	Chat      *ChatClient
	Memory    *MemoryClient
	Documents *DocumentsClient
	Agents    *AgentsClient
	Tasks     *TasksClient
	Workflows *WorkflowsClient
}

// New creates a new API client pointing at baseURL.
func New(baseURL string, opts ...Option) *Client {
	cfg := defaultConfig(baseURL)
	for _, o := range opts {
		o(&cfg)
	}

	c := &Client{cfg: cfg}
	c.Auth = &AuthClient{c: c}
	c.Chat = &ChatClient{c: c}
	c.Memory = &MemoryClient{c: c}
	c.Documents = &DocumentsClient{c: c}
	c.Agents = &AgentsClient{c: c}
	c.Tasks = &TasksClient{c: c}
	c.Workflows = &WorkflowsClient{c: c}
	return c
}

// SetToken manually injects a token (e.g. loaded from keychain / file).
func (c *Client) SetToken(access, refresh string, expiry time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = access
	c.refreshToken = refresh
	c.tokenExpiry = expiry
}

// Token returns the current access token.
func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

// request sends an authenticated HTTP request and decodes the JSON response into out.
// Pass out=nil to discard the response body.
func (c *Client) request(ctx context.Context, method, path string, body, out any) error {
	return c.do(ctx, func() error {
		return c.requestOnce(ctx, method, path, body, out)
	})
}

func (c *Client) requestOnce(ctx context.Context, method, path string, body, out any) error {
	u, err := url.JoinPath(c.cfg.BaseURL, path)
	if err != nil {
		return err
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	c.setAuth(req)

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.decodeError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// rawRequest returns the response body directly (caller must close it).
// Used for SSE and other streaming responses.
func (c *Client) rawRequest(ctx context.Context, method, path string, body any, extraHeaders map[string]string) (*http.Response, error) {
	u, err := url.JoinPath(c.cfg.BaseURL, path)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	c.setAuth(req)

	// Use a client without timeout for streaming (context controls lifetime)
	streamClient := &http.Client{Transport: c.cfg.HTTPClient.Transport}
	return streamClient.Do(req)
}

func (c *Client) setAuth(req *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
		return
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
}

func (c *Client) decodeError(resp *http.Response) error {
	apiErr := &APIError{StatusCode: resp.StatusCode}
	if err := json.NewDecoder(resp.Body).Decode(apiErr); err != nil {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}
	return apiErr
}

func (c *Client) url(path string, query url.Values) string {
	u, _ := url.JoinPath(c.cfg.BaseURL, path)
	if len(query) > 0 {
		return u + "?" + query.Encode()
	}
	return u
}
