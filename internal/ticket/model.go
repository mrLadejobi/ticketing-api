package ticket

import "time"

// TicketStatus represents the current stage of a ticket.
type TicketStatus string

const (
	StatusTodo       TicketStatus = "TODO"
	StatusInProgress TicketStatus = "IN_PROGRESS"
	StatusDone       TicketStatus = "DONE"
)

// TicketPriority represents the urgency of a ticket.
type TicketPriority string

const (
	PriorityLow    TicketPriority = "LOW"
	PriorityMedium TicketPriority = "MEDIUM"
	PriorityHigh   TicketPriority = "HIGH"
	PriorityUrgent TicketPriority = "URGENT"
)

// Ticket is our core domain entity.
type Ticket struct {
	ID          int            `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      TicketStatus   `json:"status"`
	Priority    TicketPriority `json:"priority"`
	Assignee    string         `json:"assignee,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// CreateTicketRequest defines the incoming payload when creating a ticket.
type CreateTicketRequest struct {
	Title       string         `json:"title" binding:"required,min=3,max=100"`
	Description string         `json:"description" binding:"required"`
	Priority    TicketPriority `json:"priority" binding:"required,oneof=LOW MEDIUM HIGH URGENT"`
	Assignee    string         `json:"assignee"`
}

// UpdateTicketRequest defines optional fields for a ticket update.
type UpdateTicketRequest struct {
	Title       *string         `json:"title" binding:"omitempty,min=3,max=100"`
	Description *string         `json:"description"`
	Status      *TicketStatus   `json:"status" binding:"omitempty,oneof=TODO IN_PROGRESS DONE"`
	Priority    *TicketPriority `json:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	Assignee    *string         `json:"assignee"`
}

// ListTicketsFilter defines query parameters for filtering and paginating tickets.
type ListTicketsFilter struct {
	Status   *TicketStatus   `form:"status" binding:"omitempty,oneof=TODO IN_PROGRESS DONE"`
	Priority *TicketPriority `form:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	Assignee *string         `form:"assignee"`
	Search   *string         `form:"search"`
	Limit    int             `form:"limit,default=20" binding:"min=1,max=100"`
	Offset   int             `form:"offset,default=0" binding:"min=0"`
}
