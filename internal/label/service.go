package label

import (
	"context"
	"regexp"
	"strings"
)

var hexColorRegex = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateProjectLabel(ctx context.Context, projectID int, name string, colorHex string) (*Label, error) {
	name = strings.TrimSpace(name)
	colorHex = strings.TrimSpace(colorHex)

	if !hexColorRegex.MatchString(colorHex) {
		return nil, ErrInvalidColor
	}
	if !strings.HasPrefix(colorHex, "#") {
		colorHex = "#" + colorHex
	}
	colorHex = strings.ToUpper(colorHex)

	l := &Label{
		ProjectID: projectID,
		Name:      name,
		ColorHex:  colorHex,
	}

	return s.repo.CreateProjectLabel(ctx, l)
}

func (s *Service) ListProjectLabels(ctx context.Context, projectID int) ([]Label, error) {
	return s.repo.ListProjectLabels(ctx, projectID)
}

func (s *Service) AddTicketLabel(ctx context.Context, ticketID int, labelID int) error {
	return s.repo.AddTicketLabel(ctx, ticketID, labelID)
}

func (s *Service) RemoveTicketLabel(ctx context.Context, ticketID int, labelID int) error {
	return s.repo.RemoveTicketLabel(ctx, ticketID, labelID)
}

func (s *Service) ListTicketLabels(ctx context.Context, ticketID int) ([]Label, error) {
	return s.repo.ListTicketLabels(ctx, ticketID)
}

func (s *Service) GetProjectLabelByID(ctx context.Context, id int) (*Label, error) {
	return s.repo.GetProjectLabelByID(ctx, id)
}

