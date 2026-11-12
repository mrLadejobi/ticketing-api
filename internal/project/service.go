package project

import (
	"context"
	"fmt"

	"ticketing-api/internal/auth"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateProject(ctx context.Context, creatorID int, req CreateProjectRequest) (*Project, error) {
	var creatorPtr *int
	if creatorID > 0 {
		creatorPtr = &creatorID
	}

	p := &Project{
		TeamID:          req.TeamID,
		CreatorID:       creatorPtr,
		Name:            req.Name,
		Prefix:          req.Prefix,
		Description:     req.Description,
		Status:          req.Status,
		StartDate:       req.StartDate,
		DueDate:         req.DueDate,
		ProgressPercent: req.ProgressPercent,
	}

	created, err := s.repo.CreateProject(ctx, p)
	if err != nil {
		return nil, err
	}

	if creatorID > 0 {
		_, _ = s.repo.AddMember(ctx, &ProjectMember{
			ProjectID: created.ID,
			UserID:    creatorID,
			Role:      RoleOwner,
			AddedBy:   creatorPtr,
		})
	}

	_, _ = s.repo.RecordLog(ctx, &ProjectLog{
		ProjectID: created.ID,
		ActorID:   creatorPtr,
		Action:    "PROJECT_CREATED",
		Summary:   fmt.Sprintf("Project %s (%s) created", created.Name, created.Prefix),
	})

	return created, nil
}

func (s *Service) UpdateProject(ctx context.Context, id, actorID int, req UpdateProjectRequest) (*Project, error) {
	updated, err := s.repo.UpdateProject(ctx, id, req)
	if err != nil {
		return nil, err
	}

	var actorPtr *int
	if actorID > 0 {
		actorPtr = &actorID
	}
	_, _ = s.repo.RecordLog(ctx, &ProjectLog{
		ProjectID: id,
		ActorID:   actorPtr,
		Action:    "PROJECT_UPDATED",
		Summary:   fmt.Sprintf("Project %s updated (status: %s)", updated.Prefix, updated.Status),
	})

	return updated, nil
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

func (s *Service) AddMember(ctx context.Context, projectID, actorID int, req AddMemberRequest) (*ProjectMember, error) {
	if _, err := s.repo.GetProjectByID(ctx, projectID); err != nil {
		return nil, err
	}

	var actorPtr *int
	if actorID > 0 {
		actorPtr = &actorID
	}

	pm := &ProjectMember{
		ProjectID: projectID,
		UserID:    req.UserID,
		Role:      req.Role,
		AddedBy:   actorPtr,
	}
	added, err := s.repo.AddMember(ctx, pm)
	if err != nil {
		return nil, err
	}

	_, _ = s.repo.RecordLog(ctx, &ProjectLog{
		ProjectID: projectID,
		ActorID:   actorPtr,
		Action:    "MEMBER_ADDED",
		Summary:   fmt.Sprintf("User #%d added to project as %s", req.UserID, added.Role),
	})

	return added, nil
}

// RemoveMember enforces that only the project creator (or an ADMIN) can remove members from a project.
func (s *Service) RemoveMember(ctx context.Context, projectID, targetUserID, requesterID int, requesterRole auth.Role) error {
	p, err := s.repo.GetProjectByID(ctx, projectID)
	if err != nil {
		return err
	}

	isCreator := p.CreatorID != nil && *p.CreatorID == requesterID
	if !isCreator && requesterRole != auth.RoleAdmin {
		return ErrForbidden
	}

	if err := s.repo.RemoveMember(ctx, projectID, targetUserID); err != nil {
		return err
	}

	_, _ = s.repo.RecordLog(ctx, &ProjectLog{
		ProjectID: projectID,
		ActorID:   &requesterID,
		Action:    "MEMBER_REMOVED",
		Summary:   fmt.Sprintf("User #%d removed from project", targetUserID),
	})

	return nil
}

func (s *Service) ListMembers(ctx context.Context, projectID int) ([]ProjectMember, error) {
	return s.repo.ListMembers(ctx, projectID)
}

func (s *Service) ListMemberUserIDs(ctx context.Context, projectID int) ([]int, error) {
	members, err := s.repo.ListMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.UserID)
	}
	return ids, nil
}

func (s *Service) RecordProjectLog(ctx context.Context, projectID int, actorID, ticketID *int, action, summary string) error {
	_, err := s.repo.RecordLog(ctx, &ProjectLog{
		ProjectID: projectID,
		ActorID:   actorID,
		TicketID:  ticketID,
		Action:    action,
		Summary:   summary,
	})
	return err
}

func (s *Service) ListLogs(ctx context.Context, projectID, limit, offset int) ([]ProjectLog, int, error) {
	if _, err := s.repo.GetProjectByID(ctx, projectID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListLogs(ctx, projectID, limit, offset)
}
