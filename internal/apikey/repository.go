package apikey

import (
	"context"
	"sync"
	"time"
)

type Repository interface {
	Create(ctx context.Context, key *APIKey) (*APIKey, error)
	GetByHash(ctx context.Context, hash string) (*APIKey, error)
	UpdateLastUsed(ctx context.Context, id int) error
	ListByUserID(ctx context.Context, userID int) ([]APIKey, error)
	Delete(ctx context.Context, id int, userID int) error
}

type MemoryRepository struct {
	mu      sync.RWMutex
	keys    map[int]*APIKey
	hashIdx map[string]int
	nextID  int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		keys:    make(map[int]*APIKey),
		hashIdx: make(map[string]int),
		nextID:  1,
	}
}

func (r *MemoryRepository) Create(ctx context.Context, key *APIKey) (*APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key.ID = r.nextID
	r.nextID++
	key.CreatedAt = time.Now().UTC()

	r.keys[key.ID] = key
	r.hashIdx[key.KeyHash] = key.ID

	return key, nil
}

func (r *MemoryRepository) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.hashIdx[hash]
	if !exists {
		return nil, ErrNotFound
	}
	return r.keys[id], nil
}

func (r *MemoryRepository) UpdateLastUsed(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key, exists := r.keys[id]
	if !exists {
		return ErrNotFound
	}

	now := time.Now().UTC()
	key.LastUsedAt = &now
	return nil
}

func (r *MemoryRepository) ListByUserID(ctx context.Context, userID int) ([]APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []APIKey
	for _, key := range r.keys {
		if key.UserID == userID {
			result = append(result, *key)
		}
	}
	return result, nil
}

func (r *MemoryRepository) Delete(ctx context.Context, id int, userID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key, exists := r.keys[id]
	if !exists || key.UserID != userID {
		return ErrNotFound
	}

	delete(r.hashIdx, key.KeyHash)
	delete(r.keys, id)
	return nil
}

