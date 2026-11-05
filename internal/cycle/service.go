package cycle

import (
	"context"
	"math"

	"ticketing-api/internal/ticket"
)

type TicketService interface {
	BulkUpdate(ctx context.Context, req ticket.BulkUpdateTicketsRequest) ([]ticket.Ticket, error)
	ListByCycle(ctx context.Context, cycleID int) ([]ticket.Ticket, error)
}

type Service struct {
	repo          Repository
	ticketService TicketService
}

func NewService(repo Repository, ticketService TicketService) *Service {
	return &Service{
		repo:          repo,
		ticketService: ticketService,
	}
}

func (s *Service) Create(ctx context.Context, projectID int, req CreateCycleRequest) (*Cycle, error) {
	if !req.StartDate.Before(req.EndDate) {
		return nil, ErrInvalidDates
	}

	status := req.Status
	if status == "" {
		status = StatusPlanned
	}

	c := &Cycle{
		ProjectID: projectID,
		Name:      req.Name,
		Goal:      req.Goal,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Status:    status,
	}

	return s.repo.Create(ctx, c)
}

func (s *Service) GetByID(ctx context.Context, id int) (*Cycle, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListByProjectID(ctx context.Context, projectID int) ([]Cycle, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *Service) AssignTickets(ctx context.Context, cycleID int, ticketIDs []int) error {
	if _, err := s.repo.GetByID(ctx, cycleID); err != nil {
		return err
	}

	if s.ticketService != nil {
		_, err := s.ticketService.BulkUpdate(ctx, ticket.BulkUpdateTicketsRequest{
			TicketIDs: ticketIDs,
			CycleID:   &cycleID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListCycleTickets(ctx context.Context, cycleID int) ([]ticket.Ticket, error) {
	if _, err := s.repo.GetByID(ctx, cycleID); err != nil {
		return nil, err
	}
	if s.ticketService == nil {
		return []ticket.Ticket{}, nil
	}
	return s.ticketService.ListByCycle(ctx, cycleID)
}

func (s *Service) GetAnalytics(ctx context.Context, cycleID int) (*CycleAnalytics, error) {
	cycle, err := s.repo.GetByID(ctx, cycleID)
	if err != nil {
		return nil, err
	}

	tickets, err := s.ListCycleTickets(ctx, cycleID)
	if err != nil {
		return nil, err
	}

	statusBreakdown := make(map[ticket.TicketStatus]int)
	for _, st := range ticket.AllStatuses {
		statusBreakdown[st] = 0
	}

	priorityBreakdown := map[ticket.TicketPriority]int{
		ticket.PriorityLow:    0,
		ticket.PriorityMedium: 0,
		ticket.PriorityHigh:   0,
		ticket.PriorityUrgent: 0,
	}

	total := len(tickets)
	completed := 0

	for _, t := range tickets {
		statusBreakdown[t.Status]++
		priorityBreakdown[t.Priority]++
		if t.Status == ticket.StatusDone {
			completed++
		}
	}

	var percentage float64
	if total > 0 {
		percentage = math.Round((float64(completed)/float64(total))*10000) / 100
	}

	return &CycleAnalytics{
		CycleID:              cycle.ID,
		CycleName:            cycle.Name,
		TotalTickets:         total,
		CompletedTickets:     completed,
		CompletionPercentage: percentage,
		StatusBreakdown:      statusBreakdown,
		PriorityBreakdown:    priorityBreakdown,
	}, nil
}

