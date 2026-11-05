package cycle

import (
	"errors"
	"time"

	"ticketing-api/internal/ticket"
)

var (
	ErrNotFound      = errors.New("cycle not found")
	ErrInvalidDates  = errors.New("start_date must be before end_date")
	ErrAlreadyExists = errors.New("cycle with this name already exists in this project")
)

// CycleStatus represents the progression stage of a cycle / sprint.
type CycleStatus string

const (
	StatusPlanned   CycleStatus = "PLANNED"
	StatusActive    CycleStatus = "ACTIVE"
	StatusCompleted CycleStatus = "COMPLETED"
)

// Cycle represents an agile sprint / iteration timebox.
type Cycle struct {
	ID        int         `json:"id"`
	ProjectID int         `json:"project_id"`
	Name      string      `json:"name"`
	Goal      string      `json:"goal"`
	StartDate time.Time   `json:"start_date"`
	EndDate   time.Time   `json:"end_date"`
	Status    CycleStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// CreateCycleRequest defines input payload for planning an agile cycle.
type CreateCycleRequest struct {
	Name      string      `json:"name" binding:"required,min=2,max=100"`
	Goal      string      `json:"goal"`
	StartDate time.Time   `json:"start_date" binding:"required"`
	EndDate   time.Time   `json:"end_date" binding:"required"`
	Status    CycleStatus `json:"status" binding:"omitempty,oneof=PLANNED ACTIVE COMPLETED"`
}

// AssignCycleTicketsRequest defines payload for batch associating tickets to a cycle.
type AssignCycleTicketsRequest struct {
	TicketIDs []int `json:"ticket_ids" binding:"required,min=1"`
}

// CycleAnalytics calculates velocity and issue distribution metrics for a cycle.
type CycleAnalytics struct {
	CycleID              int                          `json:"cycle_id"`
	CycleName            string                       `json:"cycle_name"`
	TotalTickets         int                          `json:"total_tickets"`
	CompletedTickets     int                          `json:"completed_tickets"`
	CompletionPercentage float64                      `json:"completion_percentage"`
	StatusBreakdown      map[ticket.TicketStatus]int   `json:"status_breakdown"`
	PriorityBreakdown    map[ticket.TicketPriority]int `json:"priority_breakdown"`
}

