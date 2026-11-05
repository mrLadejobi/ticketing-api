package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

type TokenManager struct {
	secretKey     []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewTokenManager(secretKey string, accessExpiry, refreshExpiry time.Duration) *TokenManager {
	if accessExpiry <= 0 {
		accessExpiry = 15 * time.Minute // Production best practice: short-lived 15 mins
	}
	if refreshExpiry <= 0 {
		refreshExpiry = 7 * 24 * time.Hour // Long-lived: 7 days
	}
	return &TokenManager{
		secretKey:     []byte(secretKey),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// TokenPair holds both the access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// GenerateTokenPair produces both a short-lived access token and a long-lived refresh token.
func (tm *TokenManager) GenerateTokenPair(user *User) (*TokenPair, error) {
	now := time.Now().UTC()

	// 1. Short-lived Access Token (15 mins)
	accessClaims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			Issuer:    "ticketing-api",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.accessExpiry)),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(tm.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 2. Long-lived Refresh Token (7 days)
	refreshClaims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			Issuer:    "ticketing-api-refresh",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.refreshExpiry)),
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(tm.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateToken verifies signature and expiry for access tokens.
func (tm *TokenManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	return tm.parseClaims(tokenString, "ticketing-api")
}

// ValidateRefreshToken verifies signature and expiry for refresh tokens.
func (tm *TokenManager) ValidateRefreshToken(tokenString string) (*JWTClaims, error) {
	return tm.parseClaims(tokenString, "ticketing-api-refresh")
}

func (tm *TokenManager) parseClaims(tokenString, expectedIssuer string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return tm.secretKey, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid || claims.Issuer != expectedIssuer {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
