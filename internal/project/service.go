package project

import (
	"context"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateProject(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	p := &Project{
		TeamID:      req.TeamID,
		Name:        req.Name,
		Prefix:      req.Prefix,
		Description: req.Description,
		Status:      req.Status,
	}
	return s.repo.CreateProject(ctx, p)
}

func (s *Service) GetProjectByID(ctx context.Context, id int) (*Project, error) {
	return s.repo.GetProjectByID(ctx, id)
}

func (s *Service) ListProjects(ctx context.Context, teamID *int) ([]Project, error) {
	return s.repo.ListProjects(ctx, teamID)
}

func (s *Service) NextIssueKey(ctx context.Context, projectID int) (string, error) {
	return s.repo.NextIssueKey(ctx, projectID)
}

