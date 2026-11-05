package team

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"ticketing-api/internal/auth"
)

var (
	ErrTeamNotFound     = errors.New("team not found")
	ErrDuplicateSlug    = errors.New("team slug already in use")
	ErrMemberNotFound   = errors.New("member not found in team")
	ErrMemberDuplicate  = errors.New("user is already a member of this team")
)

type Repository interface {
	CreateTeam(ctx context.Context, t *Team) (*Team, error)
	GetTeamByID(ctx context.Context, id int) (*Team, error)
	GetTeamBySlug(ctx context.Context, slug string) (*Team, error)
	ListTeams(ctx context.Context) ([]Team, error)
	AddMember(ctx context.Context, tm *TeamMember) (*TeamMember, error)
	RemoveMember(ctx context.Context, teamID, userID int) error
	ListMembers(ctx context.Context, teamID int) ([]TeamMember, error)
}

type MemoryRepository struct {
	mu      sync.RWMutex
	teams   map[int]*Team
	slugs   map[string]int
	members map[int][]*TeamMember // teamID -> members
	nextID  int
	userRepo auth.Repository
}

func NewMemoryRepository(userRepo auth.Repository) *MemoryRepository {
	return &MemoryRepository{
		teams:    make(map[int]*Team),
		slugs:    make(map[string]int),
		members:  make(map[int][]*TeamMember),
		nextID:   1,
		userRepo: userRepo,
	}
}

func (r *MemoryRepository) CreateTeam(ctx context.Context, t *Team) (*Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	normSlug := strings.ToLower(strings.TrimSpace(t.Slug))
	if _, exists := r.slugs[normSlug]; exists {
		return nil, ErrDuplicateSlug
	}

	now := time.Now().UTC()
	t.ID = r.nextID
	t.Slug = normSlug
	t.CreatedAt = now
	t.UpdatedAt = now

	r.teams[t.ID] = t
	r.slugs[normSlug] = t.ID
	r.nextID++

	return t, nil
}

func (r *MemoryRepository) GetTeamByID(ctx context.Context, id int) (*Team, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, exists := r.teams[id]
	if !exists {
		return nil, ErrTeamNotFound
	}
	return t, nil
}

func (r *MemoryRepository) GetTeamBySlug(ctx context.Context, slug string) (*Team, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.slugs[strings.ToLower(slug)]
	if !exists {
		return nil, ErrTeamNotFound
	}
	return r.teams[id], nil
}

func (r *MemoryRepository) ListTeams(ctx context.Context) ([]Team, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Team, 0, len(r.teams))
	for _, t := range r.teams {
		list = append(list, *t)
	}
	return list, nil
}

func (r *MemoryRepository) AddMember(ctx context.Context, tm *TeamMember) (*TeamMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.members[tm.TeamID]
	for _, m := range list {
		if m.UserID == tm.UserID {
			return nil, ErrMemberDuplicate
		}
	}

	tm.ID = len(list) + 1
	tm.CreatedAt = time.Now().UTC()

	if r.userRepo != nil {
		if u, err := r.userRepo.GetUserByID(ctx, tm.UserID); err == nil {
			summary := u.Summary()
			tm.User = &summary
		}
	}

	r.members[tm.TeamID] = append(list, tm)
	return tm, nil
}

func (r *MemoryRepository) RemoveMember(ctx context.Context, teamID, userID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.members[teamID]
	for i, m := range list {
		if m.UserID == userID {
			r.members[teamID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return ErrMemberNotFound
}

func (r *MemoryRepository) ListMembers(ctx context.Context, teamID int) ([]TeamMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.members[teamID]
	res := make([]TeamMember, 0, len(list))
	for _, m := range list {
		res = append(res, *m)
	}
	return res, nil
}

