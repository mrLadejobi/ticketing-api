package relation

import (
	"context"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, sourceID int, targetID int, relType RelationType) (*TicketRelation, error) {
	if sourceID == targetID {
		return nil, ErrSelfRelation
	}

	rel := &TicketRelation{
		SourceTicketID: sourceID,
		TargetTicketID: targetID,
		RelationType:   relType,
	}

	return s.repo.Create(ctx, rel)
}

func (s *Service) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) ListByTicketID(ctx context.Context, ticketID int) ([]TicketRelation, error) {
	return s.repo.ListByTicketID(ctx, ticketID)
}

func (s *Service) GetByID(ctx context.Context, id int) (*TicketRelation, error) {
	return s.repo.GetByID(ctx, id)
}

