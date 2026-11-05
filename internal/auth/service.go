package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type Service struct {
	repo         Repository
	tokenManager *TokenManager
}

func NewService(repo Repository, tokenManager *TokenManager) *Service {
	return &Service{
		repo:         repo,
		tokenManager: tokenManager,
	}
}

// Register creates a new user, defaults to MEMBER (or ADMIN for the first bootstrap user).
// Privilege escalation is prevented because role cannot be set via the request payload.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	userCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user count: %w", err)
	}

	role := RoleMember
	if userCount == 0 {
		role = RoleAdmin // Bootstrap first user
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		Email:        req.Email,
		PasswordHash: string(hash),
		FullName:     req.FullName,
		Role:         role,
	}

	createdUser, err := s.repo.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	pair, err := s.tokenManager.GenerateTokenPair(createdUser)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &AuthResponse{
		User:         *createdUser,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

// Login verifies password and issues fresh access + refresh tokens.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	pair, err := s.tokenManager.GenerateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &AuthResponse{
		User:         *user,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

// RefreshToken validates an existing refresh token and generates a new token pair.
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (*TokenPair, *User, error) {
	claims, err := s.tokenManager.ValidateRefreshToken(refreshTokenStr)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	newPair, err := s.tokenManager.GenerateTokenPair(user)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate refreshed token pair: %w", err)
	}

	return newPair, user, nil
}

// GetProfile retrieves user details by ID.
func (s *Service) GetProfile(ctx context.Context, userID int) (*User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// UpdateUserRole updates a user's system role (Admin only).
func (s *Service) UpdateUserRole(ctx context.Context, userID int, role Role) (*User, error) {
	return s.repo.UpdateUserRole(ctx, userID, role)
}

// ListUsers returns all users in summary projection.
func (s *Service) ListUsers(ctx context.Context) ([]UserSummary, error) {
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]UserSummary, 0, len(users))
	for _, u := range users {
		summaries = append(summaries, u.Summary())
	}
	return summaries, nil
}
