package project

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ticketing-api/internal/auth"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrDuplicatePrefix = errors.New("project prefix already in use")
	ErrMemberNotFound  = errors.New("project member not found")
	ErrMemberDuplicate = errors.New("user is already a member of this project")
	ErrForbidden       = errors.New("forbidden: only the project creator or an admin can perform this action")
)

type UserResolver interface {
	GetUserByID(ctx context.Context, id int) (*auth.User, error)
}

type Repository interface {
	CreateProject(ctx context.Context, p *Project) (*Project, error)
	UpdateProject(ctx context.Context, id int, req UpdateProjectRequest) (*Project, error)
	GetProjectByID(ctx context.Context, id int) (*Project, error)
	GetProjectByPrefix(ctx context.Context, prefix string) (*Project, error)
	ListProjects(ctx context.Context, teamID *int) ([]Project, error)
	NextIssueKey(ctx context.Context, projectID int) (string, error)

	// Project Members
	AddMember(ctx context.Context, pm *ProjectMember) (*ProjectMember, error)
	RemoveMember(ctx context.Context, projectID, userID int) error
	ListMembers(ctx context.Context, projectID int) ([]ProjectMember, error)

	// Project Logs
	RecordLog(ctx context.Context, log *ProjectLog) (*ProjectLog, error)
	ListLogs(ctx context.Context, projectID, limit, offset int) ([]ProjectLog, int, error)
}

type MemoryRepository struct {
	mu           sync.Mutex
	projects     map[int]*Project
	prefixes     map[string]int
	members      map[int]map[int]*ProjectMember // projectID -> userID -> ProjectMember
	logs         []ProjectLog
	userResolver UserResolver
	nextID       int
	nextMemberID int
	nextLogID    int
}

func NewMemoryRepository(ur ...UserResolver) *MemoryRepository {
	var resolver UserResolver
	if len(ur) > 0 {
		resolver = ur[0]
	}
	return &MemoryRepository{
		projects:     make(map[int]*Project),
		prefixes:     make(map[string]int),
		members:      make(map[int]map[int]*ProjectMember),
		logs:         make([]ProjectLog, 0),
		userResolver: resolver,
		nextID:       1,
		nextMemberID: 1,
		nextLogID:    1,
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

	cp := *p
	r.projects[p.ID] = &cp
	r.prefixes[normPrefix] = p.ID
	r.nextID++

	return &cp, nil
}

func (r *MemoryRepository) UpdateProject(ctx context.Context, id int, req UpdateProjectRequest) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Status != nil {
		p.Status = *req.Status
	}
	if req.StartDate != nil {
		p.StartDate = req.StartDate
	}
	if req.DueDate != nil {
		p.DueDate = req.DueDate
	}
	if req.ProgressPercent != nil {
		p.ProgressPercent = req.ProgressPercent
	}
	p.UpdatedAt = time.Now().UTC()

	cp := *p
	return &cp, nil
}

func (r *MemoryRepository) GetProjectByID(ctx context.Context, id int) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *MemoryRepository) GetProjectByPrefix(ctx context.Context, prefix string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.prefixes[strings.ToUpper(prefix)]
	if !exists {
		return nil, ErrProjectNotFound
	}
	cp := *r.projects[id]
	return &cp, nil
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
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

func (r *MemoryRepository) NextIssueKey(ctx context.Context, projectID int) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.projects[projectID]
	if !exists {
		return "", ErrProjectNotFound
	}
	p.IssueCounter++
	p.UpdatedAt = time.Now().UTC()
	return fmt.Sprintf("%s-%d", p.Prefix, p.IssueCounter), nil
}

func (r *MemoryRepository) AddMember(ctx context.Context, pm *ProjectMember) (*ProjectMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[pm.ProjectID]; !exists {
		return nil, ErrProjectNotFound
	}
	if r.members[pm.ProjectID] == nil {
		r.members[pm.ProjectID] = make(map[int]*ProjectMember)
	}
	if _, dup := r.members[pm.ProjectID][pm.UserID]; dup {
		return nil, ErrMemberDuplicate
	}

	if pm.Role == "" {
		pm.Role = RoleMember
	}
	pm.ID = r.nextMemberID
	r.nextMemberID++
	pm.CreatedAt = time.Now().UTC()

	if r.userResolver != nil {
		if u, err := r.userResolver.GetUserByID(ctx, pm.UserID); err == nil {
			pm.User = &auth.UserSummary{
				ID:       u.ID,
				Email:    u.Email,
				FullName: u.FullName,
				Role:     u.Role,
			}
		}
	}

	cp := *pm
	r.members[pm.ProjectID][pm.UserID] = &cp
	return &cp, nil
}

func (r *MemoryRepository) RemoveMember(ctx context.Context, projectID, userID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[projectID]; !exists {
		return ErrProjectNotFound
	}
	if r.members[projectID] == nil || r.members[projectID][userID] == nil {
		return ErrMemberNotFound
	}
	delete(r.members[projectID], userID)
	return nil
}

func (r *MemoryRepository) ListMembers(ctx context.Context, projectID int) ([]ProjectMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}
	list := make([]ProjectMember, 0, len(r.members[projectID]))
	for _, m := range r.members[projectID] {
		list = append(list, *m)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

func (r *MemoryRepository) RecordLog(ctx context.Context, log *ProjectLog) (*ProjectLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.ID = r.nextLogID
	r.nextLogID++
	log.CreatedAt = time.Now().UTC()
	if log.Metadata == "" {
		log.Metadata = "{}"
	}

	if log.ActorID != nil && r.userResolver != nil {
		if u, err := r.userResolver.GetUserByID(ctx, *log.ActorID); err == nil {
			log.Actor = &auth.UserSummary{
				ID:       u.ID,
				Email:    u.Email,
				FullName: u.FullName,
				Role:     u.Role,
			}
		}
	}

	cp := *log
	r.logs = append(r.logs, cp)
	return &cp, nil
}

func (r *MemoryRepository) ListLogs(ctx context.Context, projectID, limit, offset int) ([]ProjectLog, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var matched []ProjectLog
	for i := len(r.logs) - 1; i >= 0; i-- {
		if r.logs[i].ProjectID == projectID {
			matched = append(matched, r.logs[i])
		}
	}

	total := len(matched)
	if offset > total {
		offset = total
	}
	if limit <= 0 {
		limit = 20
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}
