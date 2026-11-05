package apikey

import (
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("api key not found")
	ErrExpired  = errors.New("api key has expired")
	ErrInvalid  = errors.New("invalid api key")
)

// APIKey represents metadata for a personal access token.
type APIKey struct {
	ID         int        `json:"id"`
	UserID     int        `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"` // e.g. "pat_9f81a7"
	KeyHash    string     `json:"-"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateAPIKeyRequest defines input parameters for creating a personal access token.
type CreateAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required,min=2,max=100"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// CreateAPIKeyResponse returns the generated token once upon creation.
type CreateAPIKeyResponse struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"` // Raw secret, returned only on creation
	Prefix    string     `json:"prefix"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

