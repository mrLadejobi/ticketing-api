package project

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrDuplicatePrefix = errors.New("project prefix already in use")
)

type Repository interface {
	CreateProject(ctx context.Context, p *Project) (*Project, error)
	GetProjectByID(ctx context.Context, id int) (*Project, error)
	GetProjectByPrefix(ctx context.Context, prefix string) (*Project, error)
	ListProjects(ctx context.Context, teamID *int) ([]Project, error)
	NextIssueKey(ctx context.Context, projectID int) (string, error)
}

type MemoryRepository struct {
	mu       sync.Mutex
	projects map[int]*Project
	prefixes map[string]int
	nextID   int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		projects: make(map[int]*Project),
		prefixes: make(map[string]int),
		nextID:   1,
	}
}

func (r *MemoryRepository) CreateProject(ctx context.Context, p *Project) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	normPrefix := strings.ToUpper(strings.TrimSpace(p.Prefix))
	if _, exists := r.prefixes[normPrefix]; exists {
		return nil, ErrDuplicatePrefix
	}

	now := time.Now().UTC()
	p.ID = r.nextID
	p.Prefix = normPrefix
	p.IssueCounter = 0
	if p.Status == "" {
		p.Status = StatusActive
	}
	p.CreatedAt = now
	p.UpdatedAt = now

	r.projects[p.ID] = p
	r.prefixes[normPrefix] = p.ID
	r.nextID++

	return p, nil
}

func (r *MemoryRepository) GetProjectByID(ctx context.Context, id int) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}
	return p, nil
}

func (r *MemoryRepository) GetProjectByPrefix(ctx context.Context, prefix string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.prefixes[strings.ToUpper(prefix)]
	if !exists {
		return nil, ErrProjectNotFound
	}
	return r.projects[id], nil
}

func (r *MemoryRepository) ListProjects(ctx context.Context, teamID *int) ([]Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := make([]Project, 0, len(r.projects))
	for _, p := range r.projects {
		if teamID != nil && p.TeamID != *teamID {
			continue
		}
		list = append(list, *p)
	}
	return list, nil
}

// NextIssueKey atomically increments issue counter and returns e.g. "ENG-1"
func (r *MemoryRepository) NextIssueKey(ctx context.Context, projectID int) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.projects[projectID]
	if !exists {
		return "", ErrProjectNotFound
	}

	p.IssueCounter++
	p.UpdatedAt = time.Now().UTC()
	key := fmt.Sprintf("%s-%d", p.Prefix, p.IssueCounter)
	return key, nil
}

