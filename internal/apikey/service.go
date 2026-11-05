package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"ticketing-api/internal/auth"
)

type UserGetter interface {
	GetUserByID(ctx context.Context, id int) (*auth.User, error)
}

type Service struct {
	repo       Repository
	userGetter UserGetter
	mu         sync.RWMutex
	lastSeen   map[int]time.Time
}

func NewService(repo Repository, userGetter UserGetter) *Service {
	return &Service{
		repo:       repo,
		userGetter: userGetter,
		lastSeen:   make(map[int]time.Time),
	}
}

func (s *Service) CreateKey(ctx context.Context, userID int, req CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}

	rawToken := "pat_" + hex.EncodeToString(bytes)
	prefix := rawToken[:10]

	hasher := sha256.New()
	hasher.Write([]byte(rawToken))
	keyHash := hex.EncodeToString(hasher.Sum(nil))

	key := &APIKey{
		UserID:    userID,
		Name:      req.Name,
		Prefix:    prefix,
		KeyHash:   keyHash,
		ExpiresAt: req.ExpiresAt,
	}

	saved, err := s.repo.Create(ctx, key)
	if err != nil {
		return nil, err
	}

	return &CreateAPIKeyResponse{
		ID:        saved.ID,
		Name:      saved.Name,
		Token:     rawToken,
		Prefix:    saved.Prefix,
		ExpiresAt: saved.ExpiresAt,
		CreatedAt: saved.CreatedAt,
	}, nil
}

func (s *Service) shouldUpdateLastUsed(keyID int, lastUsedAt *time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	last, exists := s.lastSeen[keyID]
	if !exists && lastUsedAt != nil {
		last = *lastUsedAt
		s.lastSeen[keyID] = last
	}

	if !exists || time.Since(last) > 5*time.Minute {
		s.lastSeen[keyID] = time.Now().UTC()
		return true
	}
	return false
}

func (s *Service) ValidateKey(ctx context.Context, rawToken string) (*auth.User, error) {
	hasher := sha256.New()
	hasher.Write([]byte(rawToken))
	keyHash := hex.EncodeToString(hasher.Sum(nil))

	key, err := s.repo.GetByHash(ctx, keyHash)
	if err != nil {
		return nil, ErrInvalid
	}

	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrExpired
	}

	// Throttle last_used_at database writes to at most once every 5 minutes per key.
	// Update is performed asynchronously in a background goroutine with a 3-second timeout.
	if s.shouldUpdateLastUsed(key.ID, key.LastUsedAt) {
		baseCtx := ctx
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(baseCtx), 3*time.Second)
		go func(keyID int) {
			defer cancel()
			_ = s.repo.UpdateLastUsed(bgCtx, keyID)
		}(key.ID)
	}

	if s.userGetter != nil {
		return s.userGetter.GetUserByID(ctx, key.UserID)
	}

	return nil, ErrNotFound
}

func (s *Service) ListKeys(ctx context.Context, userID int) ([]APIKey, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *Service) DeleteKey(ctx context.Context, id int, userID int) error {
	return s.repo.Delete(ctx, id, userID)
}

