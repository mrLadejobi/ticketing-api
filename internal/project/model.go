package project

import (
	"time"

	"ticketing-api/internal/auth"
)

type Status string

const (
	StatusPlanned    Status = "PLANNED"
	StatusStarted    Status = "STARTED"
	StatusInProgress Status = "IN_PROGRESS"
	StatusOnHold     Status = "ON_HOLD"
	StatusActive     Status = "ACTIVE"
	StatusPaused     Status = "PAUSED"
	StatusCompleted  Status = "COMPLETED"
)

type MemberRole string

const (
	RoleOwner  MemberRole = "OWNER"
	RoleMember MemberRole = "MEMBER"
	RoleViewer MemberRole = "VIEWER"
)

type Project struct {
	ID              int        `json:"id"`
	TeamID          int        `json:"team_id"`
	CreatorID       *int       `json:"creator_id,omitempty"`
	Name            string     `json:"name"`
	Prefix          string     `json:"prefix"`
	Description     string     `json:"description"`
	IssueCounter    int        `json:"issue_counter"`
	Status          Status     `json:"status"`
	StartDate       *time.Time `json:"start_date,omitempty"`
	DueDate         *time.Time `json:"due_date,omitempty"`
	ProgressPercent *int       `json:"progress_percent,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CreateProjectRequest struct {
	TeamID          int        `json:"team_id" binding:"required,min=1"`
	Name            string     `json:"name" binding:"required,min=2,max=100"`
	Prefix          string     `json:"prefix" binding:"required,min=2,max=5"`
	Description     string     `json:"description"`
	Status          Status     `json:"status" binding:"omitempty,oneof=PLANNED STARTED IN_PROGRESS ON_HOLD ACTIVE PAUSED COMPLETED"`
	StartDate       *time.Time `json:"start_date"`
	DueDate         *time.Time `json:"due_date"`
	ProgressPercent *int       `json:"progress_percent" binding:"omitempty,min=0,max=100"`
}

type UpdateProjectRequest struct {
	Name            *string    `json:"name" binding:"omitempty,min=2,max=100"`
	Description     *string    `json:"description"`
	Status          *Status    `json:"status" binding:"omitempty,oneof=PLANNED STARTED IN_PROGRESS ON_HOLD ACTIVE PAUSED COMPLETED"`
	StartDate       *time.Time `json:"start_date"`
	DueDate         *time.Time `json:"due_date"`
	ProgressPercent *int       `json:"progress_percent" binding:"omitempty,min=0,max=100"`
}

type ProjectMember struct {
	ID        int               `json:"id"`
	ProjectID int               `json:"project_id"`
	UserID    int               `json:"user_id"`
	User      *auth.UserSummary `json:"user,omitempty"`
	Role      MemberRole        `json:"role"`
	AddedBy   *int              `json:"added_by,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type AddMemberRequest struct {
	UserID int        `json:"user_id" binding:"required,min=1"`
	Role   MemberRole `json:"role" binding:"omitempty,oneof=OWNER MEMBER VIEWER"`
}

type ProjectLog struct {
	ID        int               `json:"id"`
	ProjectID int               `json:"project_id"`
	ActorID   *int              `json:"actor_id,omitempty"`
	Actor     *auth.UserSummary `json:"actor,omitempty"`
	TicketID  *int              `json:"ticket_id,omitempty"`
	Action    string            `json:"action"`
	Summary   string            `json:"summary"`
	Metadata  string            `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
}

type TimelineItem struct {
	TicketID        int        `json:"ticket_id"`
	Key             string     `json:"key"`
	Title           string     `json:"title"`
	TicketType      string     `json:"ticket_type"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	ProgressPercent *int       `json:"progress_percent,omitempty"`
	DueDate         *time.Time `json:"due_date,omitempty"`
	IsOverdue       bool       `json:"is_overdue"`
	AssigneeID      *int       `json:"assignee_id,omitempty"`
}

type TimelineResponse struct {
	Project Project        `json:"project"`
	Items   []TimelineItem `json:"items"`
}

type BoardColumn struct {
	Status  string `json:"status"`
	Count   int    `json:"count"`
	Tickets any    `json:"tickets"`
}

type BoardResponse struct {
	Project Project       `json:"project"`
	Columns []BoardColumn `json:"columns"`
}
