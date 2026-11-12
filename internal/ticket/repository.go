package ticket

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"ticketing-api/internal/auth"
)

var (
	ErrNotFound               = errors.New("ticket not found")
	ErrOptimisticLockConflict = errors.New("optimistic lock conflict: record has been updated by another user")
)

type Repository interface {
	Create(ctx context.Context, ticket *Ticket) (*Ticket, error)
	List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error)
	GetByIDOrKey(ctx context.Context, idOrKey string) (*Ticket, error)
	Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error)
	BulkUpdate(ctx context.Context, req BulkUpdateTicketsRequest) ([]Ticket, error)
	SoftDelete(ctx context.Context, id int) error
	Restore(ctx context.Context, id int) (*Ticket, error)
	ListByProject(ctx context.Context, projectID int) ([]Ticket, error)
	ListSubtasks(ctx context.Context, parentID int) ([]Ticket, error)
	ListByCycle(ctx context.Context, cycleID int) ([]Ticket, error)
}

type MemoryRepository struct {
	mu       sync.RWMutex
	tickets  map[int]*Ticket
	keyIndex map[string]int
	nextID   int
	userRepo auth.Repository
}

func NewMemoryRepository(userRepo auth.Repository) *MemoryRepository {
	return &MemoryRepository{
		tickets:  make(map[int]*Ticket),
		keyIndex: make(map[string]int),
		nextID:   1,
		userRepo: userRepo,
	}
}

func (r *MemoryRepository) Create(ctx context.Context, t *Ticket) (*Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	t.ID = r.nextID
	if t.Status == "" {
		t.Status = StatusTodo
	}
	if t.TicketType == "" {
		t.TicketType = TypeTask
	}
	t.Version = 1
	t.CreatedAt = now
	t.UpdatedAt = now

	r.hydrateUsers(ctx, t)

	r.tickets[t.ID] = t
	r.keyIndex[strings.ToUpper(t.Key)] = t.ID
	r.nextID++

	return t, nil
}

func (r *MemoryRepository) List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []Ticket
	for _, t := range r.tickets {
		if !filter.IncludeDeleted && t.DeletedAt != nil {
			continue
		}
		if filter.ProjectID != nil && t.ProjectID != *filter.ProjectID {
			continue
		}
		if filter.CycleID != nil {
			if t.CycleID == nil || *t.CycleID != *filter.CycleID {
				continue
			}
		}
		if filter.Status != nil && t.Status != *filter.Status {
			continue
		}
		if filter.TicketType != nil && t.TicketType != *filter.TicketType {
			continue
		}
		if filter.Priority != nil && t.Priority != *filter.Priority {
			continue
		}
		if filter.AssigneeID != nil {
			if t.AssigneeID == nil || *t.AssigneeID != *filter.AssigneeID {
				continue
			}
		}
		if filter.ReporterID != nil && t.ReporterID != *filter.ReporterID {
			continue
		}
		if filter.Search != nil && *filter.Search != "" {
			term := strings.ToLower(*filter.Search)
			if !strings.Contains(strings.ToLower(t.Title), term) &&
				!strings.Contains(strings.ToLower(t.Description), term) &&
				!strings.Contains(strings.ToLower(t.Key), term) {
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

func (r *MemoryRepository) GetByIDOrKey(ctx context.Context, idOrKey string) (*Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var t *Ticket
	if id, err := strconv.Atoi(idOrKey); err == nil {
		t = r.tickets[id]
	} else {
		if id, exists := r.keyIndex[strings.ToUpper(idOrKey)]; exists {
			t = r.tickets[id]
		}
	}

	if t == nil {
		return nil, ErrNotFound
	}

	// Return a copy so caller modifications or repository in-memory updates do not mutate returned reference
	ticketCopy := *t
	return &ticketCopy, nil
}

func (r *MemoryRepository) Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, exists := r.tickets[id]
	if !exists || t.DeletedAt != nil {
		return nil, ErrNotFound
	}

	// Optimistic locking verification
	if req.Version != nil && *req.Version != t.Version {
		return nil, ErrOptimisticLockConflict
	}

	if req.Title != nil {
		t.Title = *req.Title
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.WorkSummary != nil {
		t.WorkSummary = *req.WorkSummary
	}
	if req.TicketType != nil {
		t.TicketType = *req.TicketType
	}
	if req.Status != nil {
		t.Status = *req.Status
	}
	if req.Priority != nil {
		t.Priority = *req.Priority
	}
	if req.ProgressPercent != nil {
		t.ProgressPercent = req.ProgressPercent
	}
	if req.DueDate != nil {
		t.DueDate = req.DueDate
	}
	if req.AssigneeID != nil {
		t.AssigneeID = req.AssigneeID
	}
	if req.ParentID != nil {
		t.ParentID = req.ParentID
	}
	if req.CycleID != nil {
		t.CycleID = req.CycleID
	}

	t.Version++
	t.UpdatedAt = time.Now().UTC()
	r.hydrateUsers(ctx, t)

	return t, nil
}

func (r *MemoryRepository) SoftDelete(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, exists := r.tickets[id]
	if !exists || t.DeletedAt != nil {
		return ErrNotFound
	}

	now := time.Now().UTC()
	t.DeletedAt = &now
	t.UpdatedAt = now
	return nil
}

func (r *MemoryRepository) Restore(ctx context.Context, id int) (*Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, exists := r.tickets[id]
	if !exists {
		return nil, ErrNotFound
	}

	t.DeletedAt = nil
	t.UpdatedAt = time.Now().UTC()
	return t, nil
}

func (r *MemoryRepository) ListByProject(ctx context.Context, projectID int) ([]Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []Ticket
	for _, t := range r.tickets {
		if t.ProjectID == projectID && t.DeletedAt == nil {
			list = append(list, *t)
		}
	}
	return list, nil
}

func (r *MemoryRepository) ListSubtasks(ctx context.Context, parentID int) ([]Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []Ticket
	for _, t := range r.tickets {
		if t.DeletedAt == nil && t.ParentID != nil && *t.ParentID == parentID {
			list = append(list, *t)
		}
	}
	return list, nil
}

func (r *MemoryRepository) BulkUpdate(ctx context.Context, req BulkUpdateTicketsRequest) ([]Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	var updated []Ticket
	for _, id := range req.TicketIDs {
		t, exists := r.tickets[id]
		if !exists || t.DeletedAt != nil {
			continue
		}
		if req.Status != nil {
			t.Status = *req.Status
		}
		if req.Priority != nil {
			t.Priority = *req.Priority
		}
		if req.AssigneeID != nil {
			t.AssigneeID = req.AssigneeID
		}
		if req.CycleID != nil {
			t.CycleID = req.CycleID
		}
		t.Version++
		t.UpdatedAt = now
		r.hydrateUsers(ctx, t)
		updated = append(updated, *t)
	}
	return updated, nil
}

func (r *MemoryRepository) ListByCycle(ctx context.Context, cycleID int) ([]Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []Ticket
	for _, t := range r.tickets {
		if t.DeletedAt == nil && t.CycleID != nil && *t.CycleID == cycleID {
			list = append(list, *t)
		}
	}
	return list, nil
}

func (r *MemoryRepository) hydrateUsers(ctx context.Context, t *Ticket) {
	if r.userRepo == nil {
		return
	}
	if rep, err := r.userRepo.GetUserByID(ctx, t.ReporterID); err == nil {
		s := rep.Summary()
		t.Reporter = &s
	}
	if t.AssigneeID != nil {
		if ass, err := r.userRepo.GetUserByID(ctx, *t.AssigneeID); err == nil {
			s := ass.Summary()
			t.Assignee = &s
		}
	}
}
