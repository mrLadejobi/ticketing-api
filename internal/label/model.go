package label

import (
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("label not found")
	ErrAlreadyExists  = errors.New("label with this name already exists in this project")
	ErrAlreadyApplied = errors.New("label is already attached to this ticket")
	ErrInvalidColor   = errors.New("color_hex must be a valid hex color code (e.g. #EF4444)")
)

// Label represents a project-scoped categorization tag.
type Label struct {
	ID        int       `json:"id"`
	ProjectID int       `json:"project_id"`
	Name      string    `json:"name"`
	ColorHex  string    `json:"color_hex"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateLabelRequest defines input for creating a new project label.
type CreateLabelRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=50"`
	ColorHex string `json:"color_hex" binding:"required,min=4,max=10"` // e.g. #FF5733
}

// AddTicketLabelRequest defines input for attaching an existing label to a ticket.
type AddTicketLabelRequest struct {
	LabelID int `json:"label_id" binding:"required,min=1"`
}

