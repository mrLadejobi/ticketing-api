package ticket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("ticket not found")
)

// Repository defines storage operations with context cancellation support.
type Repository interface {
	Create(ctx context.Context, ticket *Ticket) (*Ticket, error)
	List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error)
	GetByID(ctx context.Context, id int) (*Ticket, error)
	Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error)
	Delete(ctx context.Context, id int) error
}

// MemoryRepository is a thread-safe in-memory store adhering to Repository.
type MemoryRepository struct {
	mu      sync.RWMutex
	tickets map[int]*Ticket
	nextID  int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		tickets: make(map[int]*Ticket),
		nextID:  1,
	}
}

func (r *MemoryRepository) Create(ctx context.Context, t *Ticket) (*Ticket, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	t.ID = r.nextID
	t.Status = StatusTodo
	t.CreatedAt = now
	t.UpdatedAt = now

	r.tickets[t.ID] = t
	r.nextID++

	return t, nil
}

func (r *MemoryRepository) List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error) {
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []Ticket
	for _, t := range r.tickets {
		if filter.Status != nil && t.Status != *filter.Status {
			continue
		}
		if filter.Priority != nil && t.Priority != *filter.Priority {
			continue
		}
		if filter.Assignee != nil && !strings.EqualFold(t.Assignee, *filter.Assignee) {
			continue
		}
		if filter.Search != nil && *filter.Search != "" {
			term := strings.ToLower(*filter.Search)
			if !strings.Contains(strings.ToLower(t.Title), term) &&
				!strings.Contains(strings.ToLower(t.Description), term) {
				continue
			}
		}
		matched = append(matched, *t)
	}

	total := len(matched)
	offset := filter.Offset
	if offset > total {
		offset = total
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return matched[offset:end], total, nil
}

func (r *MemoryRepository) GetByID(ctx context.Context, id int) (*Ticket, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	t, exists := r.tickets[id]
	if !exists {
		return nil, ErrNotFound
	}
	return t, nil
}

func (r *MemoryRepository) Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	t, exists := r.tickets[id]
	if !exists {
		return nil, ErrNotFound
	}

	if req.Title != nil {
		t.Title = *req.Title
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.Status != nil {
		t.Status = *req.Status
	}
	if req.Priority != nil {
		t.Priority = *req.Priority
	}
	if req.Assignee != nil {
		t.Assignee = *req.Assignee
	}
	t.UpdatedAt = time.Now().UTC()

	return t, nil
}

func (r *MemoryRepository) Delete(ctx context.Context, id int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tickets[id]; !exists {
		return ErrNotFound
	}
	delete(r.tickets, id)
	return nil
}
