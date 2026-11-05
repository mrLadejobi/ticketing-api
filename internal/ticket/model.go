package ticket

import (
	"time"

	"ticketing-api/internal/auth"
)

// TicketStatus represents expanded engineering sprint/issue lifecycle stages.
type TicketStatus string

const (
	StatusBacklog    TicketStatus = "BACKLOG"
	StatusTriage     TicketStatus = "TRIAGE"
	StatusTodo       TicketStatus = "TODO"
	StatusInProgress TicketStatus = "IN_PROGRESS"
	StatusInReview   TicketStatus = "IN_REVIEW"
	StatusQATesting  TicketStatus = "QA_TESTING"
	StatusBlocked    TicketStatus = "BLOCKED"
	StatusDone       TicketStatus = "DONE"
	StatusCanceled   TicketStatus = "CANCELED"
)

// AllStatuses returns the list of all valid ticket statuses in display order.
var AllStatuses = []TicketStatus{
	StatusBacklog,
	StatusTriage,
	StatusTodo,
	StatusInProgress,
	StatusInReview,
	StatusQATesting,
	StatusBlocked,
	StatusDone,
	StatusCanceled,
}

// TicketPriority represents the urgency of an issue.
type TicketPriority string

const (
	PriorityLow    TicketPriority = "LOW"
	PriorityMedium TicketPriority = "MEDIUM"
	PriorityHigh   TicketPriority = "HIGH"
	PriorityUrgent TicketPriority = "URGENT"
)

// Ticket is our enterprise domain entity with relational references, keying, and soft-delete support.
type Ticket struct {
	ID          int               `json:"id"`
	ProjectID   int               `json:"project_id"`
	ParentID    *int              `json:"parent_id,omitempty"`
	CycleID     *int              `json:"cycle_id,omitempty"`
	Key         string            `json:"key"` // e.g. "ENG-1"
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      TicketStatus      `json:"status"`
	Priority    TicketPriority    `json:"priority"`
	ReporterID  int               `json:"reporter_id"`
	Reporter    *auth.UserSummary `json:"reporter,omitempty"`
	AssigneeID  *int              `json:"assignee_id,omitempty"`
	Assignee    *auth.UserSummary `json:"assignee,omitempty"`
	Version     int               `json:"version"` // Optimistic locking
	DeletedAt   *time.Time        `json:"deleted_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CreateTicketRequest defines input parameters for creating an issue.
type CreateTicketRequest struct {
	ProjectID   int            `json:"project_id" binding:"required,min=1"`
	ParentID    *int           `json:"parent_id"`
	CycleID     *int           `json:"cycle_id"`
	Title       string         `json:"title" binding:"required,min=3,max=200"`
	Description string         `json:"description" binding:"required"`
	Priority    TicketPriority `json:"priority" binding:"required,oneof=LOW MEDIUM HIGH URGENT"`
	AssigneeID  *int           `json:"assignee_id"`
}

// UpdateTicketRequest defines parameters allowed during issue modification.
// Version is optional in body or via If-Match header to prevent concurrent lost updates.
type UpdateTicketRequest struct {
	ParentID    *int            `json:"parent_id"`
	CycleID     *int            `json:"cycle_id"`
	Title       *string         `json:"title" binding:"omitempty,min=3,max=200"`
	Description *string         `json:"description"`
	Status      *TicketStatus   `json:"status" binding:"omitempty,oneof=BACKLOG TRIAGE TODO IN_PROGRESS IN_REVIEW QA_TESTING BLOCKED DONE CANCELED"`
	Priority    *TicketPriority `json:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	AssigneeID  *int            `json:"assignee_id"`
	Version     *int            `json:"version"` // Optimistic locking check
}

// ListTicketsFilter defines multi-criteria filtering and pagination.
type ListTicketsFilter struct {
	ProjectID      *int            `form:"project_id"`
	CycleID        *int            `form:"cycle_id"`
	Status         *TicketStatus   `form:"status" binding:"omitempty,oneof=BACKLOG TRIAGE TODO IN_PROGRESS IN_REVIEW QA_TESTING BLOCKED DONE CANCELED"`
	Priority       *TicketPriority `form:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	AssigneeID     *int            `form:"assignee_id"`
	ReporterID     *int            `form:"reporter_id"`
	Search         *string         `form:"search"`
	IncludeDeleted bool            `form:"include_deleted"`
	Limit          int             `form:"limit,default=20" binding:"min=1,max=100"`
	Offset         int             `form:"offset,default=0" binding:"min=0"`
}

// BulkUpdateTicketsRequest defines parameters for updating multiple tickets atomically.
type BulkUpdateTicketsRequest struct {
	TicketIDs  []int           `json:"ticket_ids" binding:"required,min=1,max=100"`
	Status     *TicketStatus   `json:"status" binding:"omitempty,oneof=BACKLOG TRIAGE TODO IN_PROGRESS IN_REVIEW QA_TESTING BLOCKED DONE CANCELED"`
	Priority   *TicketPriority `json:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	AssigneeID *int            `json:"assignee_id"`
	CycleID    *int            `json:"cycle_id"`
}
