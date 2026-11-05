package activity

import (
	"context"
	"sync"
	"time"

	"ticketing-api/internal/auth"
)

type Repository interface {
	RecordTicketActivity(ctx context.Context, act *TicketActivity) (*TicketActivity, error)
	ListTicketActivities(ctx context.Context, ticketID int, limit, offset int) ([]TicketActivity, int, error)
	RecordAuditLog(ctx context.Context, log *AuditLog) (*AuditLog, error)
	ListAuditLogs(ctx context.Context, limit, offset int) ([]AuditLog, int, error)
}

type MemoryRepository struct {
	mu         sync.RWMutex
	activities []*TicketActivity
	auditLogs  []*AuditLog
	actNextID  int
	audNextID  int
	userRepo   auth.Repository
}

func NewMemoryRepository(userRepo auth.Repository) *MemoryRepository {
	return &MemoryRepository{
		activities: make([]*TicketActivity, 0),
		auditLogs:  make([]*AuditLog, 0),
		actNextID:  1,
		audNextID:  1,
		userRepo:   userRepo,
	}
}

func (r *MemoryRepository) RecordTicketActivity(ctx context.Context, act *TicketActivity) (*TicketActivity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	act.ID = r.actNextID
	r.actNextID++
	act.CreatedAt = time.Now().UTC()

	r.hydrateActivityActor(ctx, act)
	r.activities = append(r.activities, act)

	return act, nil
}

func (r *MemoryRepository) ListTicketActivities(ctx context.Context, ticketID int, limit, offset int) ([]TicketActivity, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []TicketActivity
	for i := len(r.activities) - 1; i >= 0; i-- {
		act := r.activities[i]
		if act.TicketID == ticketID {
			r.hydrateActivityActor(ctx, act)
			matched = append(matched, *act)
		}
	}

	total := len(matched)
	if offset > total {
		return []TicketActivity{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return matched[offset:end], total, nil
}

func (r *MemoryRepository) RecordAuditLog(ctx context.Context, log *AuditLog) (*AuditLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.ID = r.audNextID
	r.audNextID++
	log.CreatedAt = time.Now().UTC()

	r.hydrateAuditActor(ctx, log)
	r.auditLogs = append(r.auditLogs, log)

	return log, nil
}

func (r *MemoryRepository) ListAuditLogs(ctx context.Context, limit, offset int) ([]AuditLog, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []AuditLog
	for i := len(r.auditLogs) - 1; i >= 0; i-- {
		log := r.auditLogs[i]
		r.hydrateAuditActor(ctx, log)
		result = append(result, *log)
	}

	total := len(result)
	if offset > total {
		return []AuditLog{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return result[offset:end], total, nil
}

func (r *MemoryRepository) hydrateActivityActor(ctx context.Context, act *TicketActivity) {
	if r.userRepo == nil || act.ActorID == nil {
		return
	}
	if u, err := r.userRepo.GetUserByID(ctx, *act.ActorID); err == nil {
		sum := u.Summary()
		act.Actor = &sum
	}
}

func (r *MemoryRepository) hydrateAuditActor(ctx context.Context, log *AuditLog) {
	if r.userRepo == nil || log.ActorID == nil {
		return
	}
	if u, err := r.userRepo.GetUserByID(ctx, *log.ActorID); err == nil {
		sum := u.Summary()
		log.Actor = &sum
	}
}

