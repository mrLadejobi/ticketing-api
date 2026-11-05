package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role defines system-level permissions in the platform.
type Role string

const (
	RoleAdmin  Role = "ADMIN"
	RoleMember Role = "MEMBER"
	RoleViewer Role = "VIEWER"
)

// User represents an authenticated platform account.
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Omit from all JSON outputs
	FullName     string    `json:"full_name"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserSummary is a lean projection embedded in relational responses (tickets, comments, etc.).
type UserSummary struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     Role   `json:"role,omitempty"`
}

func (u *User) Summary() UserSummary {
	return UserSummary{
		ID:       u.ID,
		Email:    u.Email,
		FullName: u.FullName,
		Role:     u.Role,
	}
}

// RegisterRequest holds inputs for user registration.
// Role is intentionally excluded to prevent privilege escalation.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=100"`
	FullName string `json:"full_name" binding:"required,min=2,max=100"`
}

// UpdateUserRoleRequest is used by ADMINs to change a user's system role.
type UpdateUserRoleRequest struct {
	Role Role `json:"role" binding:"required,oneof=ADMIN MEMBER VIEWER"`
}

// LoginRequest holds credentials for user authentication.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshTokenRequest holds optional body payload when cookies are not used.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// AuthResponse is returned on successful authentication.
type AuthResponse struct {
	User         User   `json:"user"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// JWTClaims customizes JWT payload.
type JWTClaims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}
