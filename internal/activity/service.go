package activity

import (
	"context"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RecordTicketActivity(ctx context.Context, ticketID int, actorID *int, field, oldValue, newValue string) (*TicketActivity, error) {
	act := &TicketActivity{
		TicketID: ticketID,
		ActorID:  actorID,
		Field:    field,
		OldValue: oldValue,
		NewValue: newValue,
	}
	return s.repo.RecordTicketActivity(ctx, act)
}

func (s *Service) ListTicketActivities(ctx context.Context, ticketID int, limit, offset int) ([]TicketActivity, int, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repo.ListTicketActivities(ctx, ticketID, limit, offset)
}

func (s *Service) RecordAuditLog(ctx context.Context, actorID *int, action, resourceType, resourceID, payload, ip string) (*AuditLog, error) {
	log := &AuditLog{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Payload:      payload,
		IPAddress:    ip,
	}
	return s.repo.RecordAuditLog(ctx, log)
}

func (s *Service) ListAuditLogs(ctx context.Context, limit, offset int) ([]AuditLog, int, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repo.ListAuditLogs(ctx, limit, offset)
}

