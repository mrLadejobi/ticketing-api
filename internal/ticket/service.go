package ticket

import (
	"context"
	"fmt"
	"strconv"
)

type KeyGenerator interface {
	NextIssueKey(ctx context.Context, projectID int) (string, error)
}

type ActivityRecorder interface {
	RecordTicketActivity(ctx context.Context, ticketID int, actorID *int, field, oldValue, newValue string) error
}

type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Service encapsulates the business operations for tickets.
type Service struct {
	repo             Repository
	keyGenerator     KeyGenerator
	activityRecorder ActivityRecorder
	txManager        TxManager
}

func NewService(repo Repository, keyGenerator KeyGenerator) *Service {
	return &Service{
		repo:         repo,
		keyGenerator: keyGenerator,
	}
}

func (s *Service) SetActivityRecorder(recorder ActivityRecorder) {
	s.activityRecorder = recorder
}

func (s *Service) SetTxManager(txManager TxManager) {
	s.txManager = txManager
}

func (s *Service) Create(ctx context.Context, reporterID int, req CreateTicketRequest) (*Ticket, error) {
	var key string
	if s.keyGenerator != nil {
		generatedKey, err := s.keyGenerator.NextIssueKey(ctx, req.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ticket key: %w", err)
		}
		key = generatedKey
	} else {
		key = fmt.Sprintf("ISSUE-%d", req.ProjectID)
	}

	ticket := &Ticket{
		ProjectID:   req.ProjectID,
		ParentID:    req.ParentID,
		Key:         key,
		Title:       req.Title,
		Description: req.Description,
		Status:      StatusTodo,
		Priority:    req.Priority,
		ReporterID:  reporterID,
		AssigneeID:  req.AssigneeID,
	}
	return s.repo.Create(ctx, ticket)
}

func (s *Service) List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error) {
	return s.repo.List(ctx, filter)
}

func (s *Service) GetByIDOrKey(ctx context.Context, idOrKey string) (*Ticket, error) {
	return s.repo.GetByIDOrKey(ctx, idOrKey)
}

func (s *Service) ListSubtasks(ctx context.Context, parentID int) ([]Ticket, error) {
	return s.repo.ListSubtasks(ctx, parentID)
}

func (s *Service) Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error) {
	if s.txManager != nil {
		var updated *Ticket
		err := s.txManager.RunInTx(ctx, func(txCtx context.Context) error {
			var err error
			updated, err = s.updateInternal(txCtx, id, req)
			return err
		})
		if err != nil {
			return nil, err
		}
		return updated, nil
	}
	return s.updateInternal(ctx, id, req)
}

func (s *Service) updateInternal(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error) {
	var old *Ticket
	if s.activityRecorder != nil {
		if cur, err := s.repo.GetByIDOrKey(ctx, strconv.Itoa(id)); err == nil && cur != nil {
			cp := *cur
			old = &cp
		}
	}

	updated, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}

	if s.activityRecorder != nil && old != nil {
		var actorID *int
		if val := ctx.Value("user_id"); val != nil {
			if uid, ok := val.(int); ok {
				actorID = &uid
			}
		}

		if req.Status != nil && string(*req.Status) != string(old.Status) {
			if err := s.activityRecorder.RecordTicketActivity(ctx, id, actorID, "status", string(old.Status), string(*req.Status)); err != nil {
				return nil, fmt.Errorf("failed to record status activity: %w", err)
			}
		}
		if req.Priority != nil && string(*req.Priority) != string(old.Priority) {
			if err := s.activityRecorder.RecordTicketActivity(ctx, id, actorID, "priority", string(old.Priority), string(*req.Priority)); err != nil {
				return nil, fmt.Errorf("failed to record priority activity: %w", err)
			}
		}
		if req.AssigneeID != nil {
			oldAss := ""
			if old.AssigneeID != nil {
				oldAss = strconv.Itoa(*old.AssigneeID)
			}
			newAss := strconv.Itoa(*req.AssigneeID)
			if oldAss != newAss {
				if err := s.activityRecorder.RecordTicketActivity(ctx, id, actorID, "assignee_id", oldAss, newAss); err != nil {
					return nil, fmt.Errorf("failed to record assignee activity: %w", err)
				}
			}
		}
		if req.CycleID != nil {
			oldCyc := ""
			if old.CycleID != nil {
				oldCyc = strconv.Itoa(*old.CycleID)
			}
			newCyc := strconv.Itoa(*req.CycleID)
			if oldCyc != newCyc {
				if err := s.activityRecorder.RecordTicketActivity(ctx, id, actorID, "cycle_id", oldCyc, newCyc); err != nil {
					return nil, fmt.Errorf("failed to record cycle activity: %w", err)
				}
			}
		}
	}

	return updated, nil
}

func (s *Service) BulkUpdate(ctx context.Context, req BulkUpdateTicketsRequest) ([]Ticket, error) {
	if s.txManager != nil {
		var updated []Ticket
		err := s.txManager.RunInTx(ctx, func(txCtx context.Context) error {
			var err error
			updated, err = s.bulkUpdateInternal(txCtx, req)
			return err
		})
		if err != nil {
			return nil, err
		}
		return updated, nil
	}
	return s.bulkUpdateInternal(ctx, req)
}

func (s *Service) bulkUpdateInternal(ctx context.Context, req BulkUpdateTicketsRequest) ([]Ticket, error) {
	var oldTickets map[int]*Ticket
	if s.activityRecorder != nil {
		oldTickets = make(map[int]*Ticket)
		for _, id := range req.TicketIDs {
			if t, err := s.repo.GetByIDOrKey(ctx, strconv.Itoa(id)); err == nil && t != nil {
				cp := *t
				oldTickets[id] = &cp
			}
		}
	}

	updated, err := s.repo.BulkUpdate(ctx, req)
	if err != nil {
		return nil, err
	}

	if s.activityRecorder != nil {
		var actorID *int
		if val := ctx.Value("user_id"); val != nil {
			if uid, ok := val.(int); ok {
				actorID = &uid
			}
		}

		for _, t := range updated {
			old := oldTickets[t.ID]
			if old == nil {
				continue
			}
			if req.Status != nil && string(*req.Status) != string(old.Status) {
				if err := s.activityRecorder.RecordTicketActivity(ctx, t.ID, actorID, "status", string(old.Status), string(*req.Status)); err != nil {
					return nil, fmt.Errorf("failed to record bulk status activity: %w", err)
				}
			}
			if req.Priority != nil && string(*req.Priority) != string(old.Priority) {
				if err := s.activityRecorder.RecordTicketActivity(ctx, t.ID, actorID, "priority", string(old.Priority), string(*req.Priority)); err != nil {
					return nil, fmt.Errorf("failed to record bulk priority activity: %w", err)
				}
			}
			if req.AssigneeID != nil {
				oldAss := ""
				if old.AssigneeID != nil {
					oldAss = strconv.Itoa(*old.AssigneeID)
				}
				newAss := strconv.Itoa(*req.AssigneeID)
				if oldAss != newAss {
					if err := s.activityRecorder.RecordTicketActivity(ctx, t.ID, actorID, "assignee_id", oldAss, newAss); err != nil {
						return nil, fmt.Errorf("failed to record bulk assignee activity: %w", err)
					}
				}
			}
			if req.CycleID != nil {
				oldCyc := ""
				if old.CycleID != nil {
					oldCyc = strconv.Itoa(*old.CycleID)
				}
				newCyc := strconv.Itoa(*req.CycleID)
				if oldCyc != newCyc {
					if err := s.activityRecorder.RecordTicketActivity(ctx, t.ID, actorID, "cycle_id", oldCyc, newCyc); err != nil {
						return nil, fmt.Errorf("failed to record bulk cycle activity: %w", err)
					}
				}
			}
		}
	}

	return updated, nil
}

func (s *Service) ListByCycle(ctx context.Context, cycleID int) ([]Ticket, error) {
	return s.repo.ListByCycle(ctx, cycleID)
}

func (s *Service) SoftDelete(ctx context.Context, id int) error {
	return s.repo.SoftDelete(ctx, id)
}

func (s *Service) Restore(ctx context.Context, id int) (*Ticket, error) {
	return s.repo.Restore(ctx, id)
}

func (s *Service) ListByProject(ctx context.Context, projectID int) ([]Ticket, error) {
	return s.repo.ListByProject(ctx, projectID)
}
