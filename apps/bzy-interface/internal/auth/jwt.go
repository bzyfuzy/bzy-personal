// Package auth handles JWT token issuance and validation.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the platform JWT payload.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

// TokenPair holds access + refresh tokens.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// JWTService manages token lifecycle.
type JWTService struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewJWTService(secret string, accessExpiry, refreshExpiry time.Duration) *JWTService {
	return &JWTService{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// Issue creates a new token pair for a user.
func (s *JWTService) Issue(userID, email string, roles []string) (*TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(s.accessExpiry)

	accessClaims := Claims{
		UserID: userID,
		Email:  email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExp),
			ID:        uuid.New().String(),
			Issuer:    "bzy-interface",
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(s.secret)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshExpiry)),
			ID:        uuid.New().String(),
			Issuer:    "bzy-interface",
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(s.secret)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExp,
	}, nil
}

// Validate parses and validates a token, returning its claims.
func (s *JWTService) Validate(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

// ─── API Key service ──────────────────────────────────────────────────────────

// APIKeyService validates static API keys (stored in Redis or DB).
type APIKeyService struct {
	// keys: key → userID (in production, store in DB with hashed keys)
	keys map[string]string
}

func NewAPIKeyService() *APIKeyService {
	return &APIKeyService{keys: make(map[string]string)}
}

func (s *APIKeyService) Validate(key string) (string, bool) {
	userID, ok := s.keys[key]
	return userID, ok
}

func (s *APIKeyService) Register(key, userID string) {
	s.keys[key] = userID
}
