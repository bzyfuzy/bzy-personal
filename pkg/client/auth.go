package client

import (
	"context"
	"fmt"
	"time"
)

// ─── Request / Response types ─────────────────────────────────────────────────

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ─── AuthClient ───────────────────────────────────────────────────────────────

type AuthClient struct{ c *Client }

// Login authenticates with email/password and stores the returned tokens.
func (a *AuthClient) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	var pair TokenPair
	if err := a.c.request(ctx, "POST", "/api/v1/auth/login",
		LoginRequest{Email: email, Password: password}, &pair); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	a.c.SetToken(pair.AccessToken, pair.RefreshToken, pair.ExpiresAt)
	return &pair, nil
}

// Refresh obtains a new access token using the stored refresh token.
func (a *AuthClient) Refresh(ctx context.Context) (*TokenPair, error) {
	a.c.mu.RLock()
	rt := a.c.refreshToken
	a.c.mu.RUnlock()

	if rt == "" {
		return nil, fmt.Errorf("no refresh token stored")
	}

	var pair TokenPair
	if err := a.c.request(ctx, "POST", "/api/v1/auth/refresh",
		RefreshRequest{RefreshToken: rt}, &pair); err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	a.c.SetToken(pair.AccessToken, pair.RefreshToken, pair.ExpiresAt)
	return &pair, nil
}

// EnsureFresh refreshes the token if it will expire within the next minute.
func (a *AuthClient) EnsureFresh(ctx context.Context) error {
	a.c.mu.RLock()
	expiry := a.c.tokenExpiry
	a.c.mu.RUnlock()

	if time.Until(expiry) > time.Minute {
		return nil
	}
	_, err := a.Refresh(ctx)
	return err
}

// Login is a top-level shortcut on Client.
func (c *Client) Login(ctx context.Context, email, password string) error {
	_, err := c.Auth.Login(ctx, email, password)
	return err
}
