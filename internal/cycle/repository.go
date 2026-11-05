package cycle

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Repository interface {
	Create(ctx context.Context, c *Cycle) (*Cycle, error)
	GetByID(ctx context.Context, id int) (*Cycle, error)
	ListByProjectID(ctx context.Context, projectID int) ([]Cycle, error)
	Update(ctx context.Context, id int, c *Cycle) (*Cycle, error)
}

type MemoryRepository struct {
	mu     sync.RWMutex
	cycles map[int]*Cycle
	nextID int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		cycles: make(map[int]*Cycle),
		nextID: 1,
	}
}

func (r *MemoryRepository) Create(ctx context.Context, c *Cycle) (*Cycle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.cycles {
		if existing.ProjectID == c.ProjectID && strings.EqualFold(existing.Name, c.Name) {
			return nil, ErrAlreadyExists
		}
	}

	c.ID = r.nextID
	r.nextID++
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now
	if c.Status == "" {
		c.Status = StatusPlanned
	}

	r.cycles[c.ID] = c
	return c, nil
}

func (r *MemoryRepository) GetByID(ctx context.Context, id int) (*Cycle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, exists := r.cycles[id]
	if !exists {
		return nil, ErrNotFound
	}
	return c, nil
}

func (r *MemoryRepository) ListByProjectID(ctx context.Context, projectID int) ([]Cycle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Cycle
	for _, c := range r.cycles {
		if c.ProjectID == projectID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (r *MemoryRepository) Update(ctx context.Context, id int, c *Cycle) (*Cycle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.cycles[id]
	if !exists {
		return nil, ErrNotFound
	}

	existing.Name = c.Name
	existing.Goal = c.Goal
	existing.StartDate = c.StartDate
	existing.EndDate = c.EndDate
	existing.Status = c.Status
	existing.UpdatedAt = time.Now().UTC()

	return existing, nil
}

