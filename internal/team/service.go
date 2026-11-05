package team

import (
	"context"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTeam(ctx context.Context, req CreateTeamRequest) (*Team, error) {
	t := &Team{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	}
	return s.repo.CreateTeam(ctx, t)
}

func (s *Service) GetTeamByID(ctx context.Context, id int) (*Team, error) {
	return s.repo.GetTeamByID(ctx, id)
}

func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	return s.repo.ListTeams(ctx)
}

func (s *Service) AddMember(ctx context.Context, teamID int, req AddMemberRequest) (*TeamMember, error) {
	tm := &TeamMember{
		TeamID: teamID,
		UserID: req.UserID,
		Role:   req.Role,
	}
	return s.repo.AddMember(ctx, tm)
}

func (s *Service) RemoveMember(ctx context.Context, teamID, userID int) error {
	return s.repo.RemoveMember(ctx, teamID, userID)
}

func (s *Service) ListMembers(ctx context.Context, teamID int) ([]TeamMember, error) {
	return s.repo.ListMembers(ctx, teamID)
}

