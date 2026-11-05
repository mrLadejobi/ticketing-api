package notification

import (
	"context"
)

type EventPublisher interface {
	PublishUserNotification(userID int, n *Notification)
}

type Service struct {
	repo      Repository
	publisher EventPublisher
}

func NewService(repo Repository, publisher ...EventPublisher) *Service {
	var pub EventPublisher
	if len(publisher) > 0 {
		pub = publisher[0]
	}
	return &Service{
		repo:      repo,
		publisher: pub,
	}
}

func (s *Service) SetPublisher(pub EventPublisher) {
	s.publisher = pub
}

func (s *Service) Notify(ctx context.Context, userID int, actorID, ticketID *int, nType NotificationType, title, body string) (*Notification, error) {
	// Do not notify self
	if actorID != nil && *actorID == userID {
		return nil, nil
	}

	n := &Notification{
		UserID:   userID,
		ActorID:  actorID,
		TicketID: ticketID,
		Type:     nType,
		Title:    title,
		Body:     body,
	}

	saved, err := s.repo.Create(ctx, n)
	if err != nil {
		return nil, err
	}

	if s.publisher != nil && saved != nil {
		s.publisher.PublishUserNotification(userID, saved)
	}

	return saved, nil
}

func (s *Service) List(ctx context.Context, filter ListNotificationsFilter) ([]Notification, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	return s.repo.List(ctx, filter)
}

func (s *Service) MarkAsRead(ctx context.Context, id, userID int) error {
	return s.repo.MarkAsRead(ctx, id, userID)
}

func (s *Service) MarkAllAsRead(ctx context.Context, userID int) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

