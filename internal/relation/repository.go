package relation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ticketing-api/internal/ticket"
)

type TicketGetter interface {
	GetByIDOrKey(ctx context.Context, idOrKey string) (*ticket.Ticket, error)
}

type Repository interface {
	Create(ctx context.Context, rel *TicketRelation) (*TicketRelation, error)
	Delete(ctx context.Context, id int) error
	GetByID(ctx context.Context, id int) (*TicketRelation, error)
	ListByTicketID(ctx context.Context, ticketID int) ([]TicketRelation, error)
}

type MemoryRepository struct {
	mu           sync.RWMutex
	relations    map[int]*TicketRelation
	nextID       int
	ticketGetter TicketGetter
}

func NewMemoryRepository(ticketGetter TicketGetter) *MemoryRepository {
	return &MemoryRepository{
		relations:    make(map[int]*TicketRelation),
		nextID:       1,
		ticketGetter: ticketGetter,
	}
}

func (r *MemoryRepository) Create(ctx context.Context, rel *TicketRelation) (*TicketRelation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rel.SourceTicketID == rel.TargetTicketID {
		return nil, ErrSelfRelation
	}

	for _, existing := range r.relations {
		if existing.SourceTicketID == rel.SourceTicketID &&
			existing.TargetTicketID == rel.TargetTicketID &&
			existing.RelationType == rel.RelationType {
			return nil, ErrRelationExists
		}
	}

	rel.ID = r.nextID
	r.nextID++
	rel.CreatedAt = time.Now().UTC()

	r.hydrateTickets(ctx, rel)
	r.relations[rel.ID] = rel

	return rel, nil
}

func (r *MemoryRepository) Delete(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.relations[id]; !exists {
		return ErrNotFound
	}
	delete(r.relations, id)
	return nil
}

func (r *MemoryRepository) GetByID(ctx context.Context, id int) (*TicketRelation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rel, exists := r.relations[id]
	if !exists {
		return nil, ErrNotFound
	}
	return rel, nil
}

func (r *MemoryRepository) ListByTicketID(ctx context.Context, ticketID int) ([]TicketRelation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []TicketRelation
	for _, rel := range r.relations {
		if rel.SourceTicketID == ticketID || rel.TargetTicketID == ticketID {
			r.hydrateTickets(ctx, rel)
			result = append(result, *rel)
		}
	}
	return result, nil
}

func (r *MemoryRepository) hydrateTickets(ctx context.Context, rel *TicketRelation) {
	if r.ticketGetter == nil {
		return
	}
	if src, err := r.ticketGetter.GetByIDOrKey(ctx, fmt.Sprintf("%d", rel.SourceTicketID)); err == nil {
		rel.SourceTicket = &TicketSummary{
			ID:       src.ID,
			Key:      src.Key,
			Title:    src.Title,
			Status:   src.Status,
			Priority: src.Priority,
		}
	}
	if tgt, err := r.ticketGetter.GetByIDOrKey(ctx, fmt.Sprintf("%d", rel.TargetTicketID)); err == nil {
		rel.TargetTicket = &TicketSummary{
			ID:       tgt.ID,
			Key:      tgt.Key,
			Title:    tgt.Title,
			Status:   tgt.Status,
			Priority: tgt.Priority,
		}
	}
}

