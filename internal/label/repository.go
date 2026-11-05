package label

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Repository interface {
	CreateProjectLabel(ctx context.Context, label *Label) (*Label, error)
	GetProjectLabelByID(ctx context.Context, id int) (*Label, error)
	ListProjectLabels(ctx context.Context, projectID int) ([]Label, error)
	AddTicketLabel(ctx context.Context, ticketID int, labelID int) error
	RemoveTicketLabel(ctx context.Context, ticketID int, labelID int) error
	ListTicketLabels(ctx context.Context, ticketID int) ([]Label, error)
}

type MemoryRepository struct {
	mu           sync.RWMutex
	labels       map[int]*Label
	ticketLabels map[int]map[int]bool // ticketID -> set of labelIDs
	nextID       int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		labels:       make(map[int]*Label),
		ticketLabels: make(map[int]map[int]bool),
		nextID:       1,
	}
}

func (r *MemoryRepository) CreateProjectLabel(ctx context.Context, l *Label) (*Label, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.labels {
		if existing.ProjectID == l.ProjectID && strings.EqualFold(existing.Name, l.Name) {
			return nil, ErrAlreadyExists
		}
	}

	l.ID = r.nextID
	r.nextID++
	l.CreatedAt = time.Now().UTC()
	r.labels[l.ID] = l

	return l, nil
}

func (r *MemoryRepository) GetProjectLabelByID(ctx context.Context, id int) (*Label, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	l, exists := r.labels[id]
	if !exists {
		return nil, ErrNotFound
	}
	return l, nil
}

func (r *MemoryRepository) ListProjectLabels(ctx context.Context, projectID int) ([]Label, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Label
	for _, l := range r.labels {
		if l.ProjectID == projectID {
			result = append(result, *l)
		}
	}
	return result, nil
}

func (r *MemoryRepository) AddTicketLabel(ctx context.Context, ticketID int, labelID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.labels[labelID]; !exists {
		return ErrNotFound
	}

	set, exists := r.ticketLabels[ticketID]
	if !exists {
		set = make(map[int]bool)
		r.ticketLabels[ticketID] = set
	}

	if set[labelID] {
		return ErrAlreadyApplied
	}

	set[labelID] = true
	return nil
}

func (r *MemoryRepository) RemoveTicketLabel(ctx context.Context, ticketID int, labelID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	set, exists := r.ticketLabels[ticketID]
	if !exists || !set[labelID] {
		return ErrNotFound
	}

	delete(set, labelID)
	return nil
}

func (r *MemoryRepository) ListTicketLabels(ctx context.Context, ticketID int) ([]Label, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	set, exists := r.ticketLabels[ticketID]
	if !exists {
		return []Label{}, nil
	}

	var result []Label
	for labelID := range set {
		if l, found := r.labels[labelID]; found {
			result = append(result, *l)
		}
	}
	return result, nil
}

