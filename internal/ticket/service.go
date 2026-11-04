package ticket

import "context"

// Service encapsulates the business operations for tickets.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req CreateTicketRequest) (*Ticket, error) {
	ticket := &Ticket{
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Assignee:    req.Assignee,
	}
	return s.repo.Create(ctx, ticket)
}

func (s *Service) List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error) {
	return s.repo.List(ctx, filter)
}

func (s *Service) GetByID(ctx context.Context, id int) (*Ticket, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
