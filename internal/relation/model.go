package relation

import (
	"errors"
	"time"

	"ticketing-api/internal/ticket"
)

var (
	ErrNotFound          = errors.New("ticket relation not found")
	ErrSelfRelation      = errors.New("cannot create a relation between a ticket and itself")
	ErrRelationExists    = errors.New("relation between these tickets already exists")
	ErrInvalidTarget     = errors.New("target ticket does not exist or is invalid")
)

// RelationType defines supported relationship links between tickets.
type RelationType string

const (
	RelationBlocks      RelationType = "BLOCKS"
	RelationIsBlockedBy RelationType = "IS_BLOCKED_BY"
	RelationRelatesTo   RelationType = "RELATES_TO"
	RelationDuplicates  RelationType = "DUPLICATES"
)

// AllRelationTypes contains all valid relation types.
var AllRelationTypes = []RelationType{
	RelationBlocks,
	RelationIsBlockedBy,
	RelationRelatesTo,
	RelationDuplicates,
}

// TicketSummary provides compact metadata for related tickets.
type TicketSummary struct {
	ID       int                   `json:"id"`
	Key      string                `json:"key"`
	Title    string                `json:"title"`
	Status   ticket.TicketStatus   `json:"status"`
	Priority ticket.TicketPriority `json:"priority"`
}

// TicketRelation represents a directional or associative link between two tickets.
type TicketRelation struct {
	ID             int            `json:"id"`
	SourceTicketID int            `json:"source_ticket_id"`
	SourceTicket   *TicketSummary `json:"source_ticket,omitempty"`
	TargetTicketID int            `json:"target_ticket_id"`
	TargetTicket   *TicketSummary `json:"target_ticket,omitempty"`
	RelationType   RelationType   `json:"relation_type"`
	CreatedAt      time.Time      `json:"created_at"`
}

// CreateRelationRequest defines the input payload to link tickets.
type CreateRelationRequest struct {
	TargetTicketIDOrKey string       `json:"target_ticket_id_or_key" binding:"required"`
	RelationType        RelationType `json:"relation_type" binding:"required,oneof=BLOCKS IS_BLOCKED_BY RELATES_TO DUPLICATES"`
}

