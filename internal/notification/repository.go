package notification

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"ticketing-api/internal/auth"
)

var (
	ErrNotFound = errors.New("notification not found")
)

type Repository interface {
	Create(ctx context.Context, n *Notification) (*Notification, error)
	List(ctx context.Context, filter ListNotificationsFilter) ([]Notification, int, error)
	MarkAsRead(ctx context.Context, id, userID int) error
	MarkAllAsRead(ctx context.Context, userID int) error
}

type UserResolver interface {
	GetUserByID(ctx context.Context, id int) (*auth.User, error)
}

type MemoryRepository struct {
	mu            sync.RWMutex
	notifications map[int]*Notification
	userResolver  UserResolver
	nextID        int
}

func NewMemoryRepository(userResolver UserResolver) *MemoryRepository {
	return &MemoryRepository{
		notifications: make(map[int]*Notification),
		userResolver:  userResolver,
		nextID:        1,
	}
}

func (r *MemoryRepository) Create(ctx context.Context, n *Notification) (*Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n.ID = r.nextID
	r.nextID++
	n.CreatedAt = time.Now().UTC()
	n.IsRead = false

	if n.ActorID != nil && r.userResolver != nil {
		if u, err := r.userResolver.GetUserByID(ctx, *n.ActorID); err == nil {
			n.Actor = &auth.UserSummary{
				ID:       u.ID,
				Email:    u.Email,
				FullName: u.FullName,
				Role:     u.Role,
			}
		}
	}

	saved := *n
	r.notifications[n.ID] = &saved

	return n, nil
}

func (r *MemoryRepository) List(ctx context.Context, filter ListNotificationsFilter) ([]Notification, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []Notification
	for _, n := range r.notifications {
		if n.UserID != filter.UserID {
			continue
		}
		if filter.UnreadOnly && n.IsRead {
			continue
		}
		matched = append(matched, *n)
	}

	// Sort chronologically descending
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

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

func (r *MemoryRepository) MarkAsRead(ctx context.Context, id, userID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, exists := r.notifications[id]
	if !exists || n.UserID != userID {
		return ErrNotFound
	}
	n.IsRead = true
	return nil
}

func (r *MemoryRepository) MarkAllAsRead(ctx context.Context, userID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, n := range r.notifications {
		if n.UserID == userID {
			n.IsRead = true
		}
	}
	return nil
}

