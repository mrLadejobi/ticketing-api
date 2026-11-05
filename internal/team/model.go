package team

import (
	"time"

	"ticketing-api/internal/auth"
)

type MemberRole string

const (
	RoleLead        MemberRole = "LEAD"
	RoleDeveloper   MemberRole = "DEVELOPER"
	RoleStakeholder MemberRole = "STAKEHOLDER"
)

type Team struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TeamMember struct {
	ID        int               `json:"id"`
	TeamID    int               `json:"team_id"`
	UserID    int               `json:"user_id"`
	Role      MemberRole        `json:"role"`
	User      *auth.UserSummary `json:"user,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type CreateTeamRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Slug        string `json:"slug" binding:"required,min=2,max=50"`
	Description string `json:"description"`
}

type AddMemberRequest struct {
	UserID int        `json:"user_id" binding:"required,min=1"`
	Role   MemberRole `json:"role" binding:"required,oneof=LEAD DEVELOPER STAKEHOLDER"`
}

