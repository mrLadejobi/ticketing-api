package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateEmail = errors.New("email already in use")
)

type Repository interface {
	CreateUser(ctx context.Context, u *User) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id int) (*User, error)
	UpdateUserRole(ctx context.Context, userID int, role Role) (*User, error)
	CountUsers(ctx context.Context) (int, error)
	ListUsers(ctx context.Context) ([]User, error)
}

// MemoryRepository provides a thread-safe in-memory store for users.
type MemoryRepository struct {
	mu     sync.RWMutex
	users  map[int]*User
	emails map[string]int
	nextID int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:  make(map[int]*User),
		emails: make(map[string]int),
		nextID: 1,
	}
}

func (r *MemoryRepository) CreateUser(ctx context.Context, u *User) (*User, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	normEmail := strings.ToLower(strings.TrimSpace(u.Email))
	if _, exists := r.emails[normEmail]; exists {
		return nil, ErrDuplicateEmail
	}

	now := time.Now().UTC()
	u.ID = r.nextID
	u.Email = normEmail
	u.CreatedAt = now
	u.UpdatedAt = now

	r.users[u.ID] = u
	r.emails[normEmail] = u.ID
	r.nextID++

	return u, nil
}

func (r *MemoryRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	normEmail := strings.ToLower(strings.TrimSpace(email))
	id, exists := r.emails[normEmail]
	if !exists {
		return nil, ErrUserNotFound
	}

	return r.users[id], nil
}

func (r *MemoryRepository) GetUserByID(ctx context.Context, id int) (*User, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (r *MemoryRepository) UpdateUserRole(ctx context.Context, userID int, role Role) (*User, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}

	user.Role = role
	user.UpdatedAt = time.Now().UTC()
	return user, nil
}

func (r *MemoryRepository) CountUsers(ctx context.Context) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.users), nil
}

func (r *MemoryRepository) ListUsers(ctx context.Context) ([]User, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]User, 0, len(r.users))
	for _, u := range r.users {
		list = append(list, *u)
	}
	return list, nil
}
