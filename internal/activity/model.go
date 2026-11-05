package activity

import (
	"time"

	"ticketing-api/internal/auth"
)

// TicketActivity represents an immutable change record on a ticket field.
type TicketActivity struct {
	ID        int               `json:"id"`
	TicketID  int               `json:"ticket_id"`
	ActorID   *int              `json:"actor_id,omitempty"`
	Actor     *auth.UserSummary `json:"actor,omitempty"`
	Field     string            `json:"field"`
	OldValue  string            `json:"old_value"`
	NewValue  string            `json:"new_value"`
	CreatedAt time.Time         `json:"created_at"`
}

// AuditLog represents a system-wide security, administrative, or mutation audit trail record.
type AuditLog struct {
	ID           int               `json:"id"`
	ActorID      *int              `json:"actor_id,omitempty"`
	Actor        *auth.UserSummary `json:"actor,omitempty"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id"`
	Payload      string            `json:"payload"`
	IPAddress    string            `json:"ip_address"`
	CreatedAt    time.Time         `json:"created_at"`
}

