package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ticketing-api/internal/auth"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateProject(ctx context.Context, p *Project) (*Project, error) {
	query := `
		INSERT INTO projects (team_id, creator_id, name, prefix, description, issue_counter, status, start_date, due_date, progress_percent, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`
	now := time.Now().UTC()
	p.Prefix = strings.ToUpper(strings.TrimSpace(p.Prefix))
	p.IssueCounter = 0
	if p.Status == "" {
		p.Status = StatusActive
	}
	p.CreatedAt = now
	p.UpdatedAt = now

	err := r.db.QueryRowContext(
		ctx, query,
		p.TeamID, p.CreatorID, p.Name, p.Prefix, p.Description, p.IssueCounter, p.Status, p.StartDate, p.DueDate, p.ProgressPercent, p.CreatedAt, p.UpdatedAt,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			return nil, ErrDuplicatePrefix
		}
		return nil, fmt.Errorf("create project failed: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) UpdateProject(ctx context.Context, id int, req UpdateProjectRequest) (*Project, error) {
	current, err := r.GetProjectByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		current.Name = *req.Name
	}
	if req.Description != nil {
		current.Description = *req.Description
	}
	if req.Status != nil {
		current.Status = *req.Status
	}
	if req.StartDate != nil {
		current.StartDate = req.StartDate
	}
	if req.DueDate != nil {
		current.DueDate = req.DueDate
	}
	if req.ProgressPercent != nil {
		current.ProgressPercent = req.ProgressPercent
	}
	current.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE projects
		SET name = $1, description = $2, status = $3, start_date = $4, due_date = $5, progress_percent = $6, updated_at = $7
		WHERE id = $8
	`
	if _, err := r.db.ExecContext(ctx, query, current.Name, current.Description, current.Status, current.StartDate, current.DueDate, current.ProgressPercent, current.UpdatedAt, id); err != nil {
		return nil, fmt.Errorf("update project failed: %w", err)
	}
	return current, nil
}

func scanProject(scanner interface{ Scan(dest ...any) error }) (*Project, error) {
	var p Project
	var creatorID, progress sql.NullInt64
	var startDate, dueDate sql.NullTime

	err := scanner.Scan(
		&p.ID, &p.TeamID, &creatorID, &p.Name, &p.Prefix, &p.Description, &p.IssueCounter, &p.Status,
		&startDate, &dueDate, &progress, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if creatorID.Valid {
		cid := int(creatorID.Int64)
		p.CreatorID = &cid
	}
	if startDate.Valid {
		t := startDate.Time
		p.StartDate = &t
	}
	if dueDate.Valid {
		t := dueDate.Time
		p.DueDate = &t
	}
	if progress.Valid {
		pr := int(progress.Int64)
		p.ProgressPercent = &pr
	}
	return &p, nil
}

func (r *PostgresRepository) GetProjectByID(ctx context.Context, id int) (*Project, error) {
	query := `SELECT id, team_id, creator_id, name, prefix, description, issue_counter, status, start_date, due_date, progress_percent, created_at, updated_at FROM projects WHERE id = $1`
	p, err := scanProject(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("get project by id failed: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) GetProjectByPrefix(ctx context.Context, prefix string) (*Project, error) {
	query := `SELECT id, team_id, creator_id, name, prefix, description, issue_counter, status, start_date, due_date, progress_percent, created_at, updated_at FROM projects WHERE UPPER(prefix) = UPPER($1)`
	p, err := scanProject(r.db.QueryRowContext(ctx, query, strings.TrimSpace(prefix)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("get project by prefix failed: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) ListProjects(ctx context.Context, teamID *int) ([]Project, error) {
	var query string
	var args []any
	if teamID != nil {
		query = `SELECT id, team_id, creator_id, name, prefix, description, issue_counter, status, start_date, due_date, progress_percent, created_at, updated_at FROM projects WHERE team_id = $1 ORDER BY id ASC`
		args = append(args, *teamID)
	} else {
		query = `SELECT id, team_id, creator_id, name, prefix, description, issue_counter, status, start_date, due_date, progress_percent, created_at, updated_at FROM projects ORDER BY id ASC`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects failed: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects failed: %w", err)
	}
	return projects, nil
}

// NextIssueKey uses atomic UPDATE ... RETURNING in PostgreSQL to safely increment issue counter
func (r *PostgresRepository) NextIssueKey(ctx context.Context, projectID int) (string, error) {
	query := `
		UPDATE projects
		SET issue_counter = issue_counter + 1, updated_at = $1
		WHERE id = $2
		RETURNING prefix, issue_counter
	`
	now := time.Now().UTC()
	var prefix string
	var counter int
	err := r.db.QueryRowContext(ctx, query, now, projectID).Scan(&prefix, &counter)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrProjectNotFound
		}
		return "", fmt.Errorf("atomic increment issue key failed: %w", err)
	}
	return fmt.Sprintf("%s-%d", prefix, counter), nil
}

func (r *PostgresRepository) AddMember(ctx context.Context, pm *ProjectMember) (*ProjectMember, error) {
	if pm.Role == "" {
		pm.Role = RoleMember
	}
	now := time.Now().UTC()
	query := `
		INSERT INTO project_members (project_id, user_id, role, added_by, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query, pm.ProjectID, pm.UserID, pm.Role, pm.AddedBy, now).Scan(&pm.ID, &pm.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			return nil, ErrMemberDuplicate
		}
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "23503") {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("add project member failed: %w", err)
	}

	userQuery := `SELECT id, email, full_name, role FROM users WHERE id = $1`
	var u auth.UserSummary
	if err := r.db.QueryRowContext(ctx, userQuery, pm.UserID).Scan(&u.ID, &u.Email, &u.FullName, &u.Role); err == nil {
		pm.User = &u
	}
	return pm, nil
}

func (r *PostgresRepository) RemoveMember(ctx context.Context, projectID, userID int) error {
	query := `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, query, projectID, userID)
	if err != nil {
		return fmt.Errorf("remove project member failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *PostgresRepository) ListMembers(ctx context.Context, projectID int) ([]ProjectMember, error) {
	query := `
		SELECT pm.id, pm.project_id, pm.user_id, pm.role, pm.added_by, pm.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM project_members pm
		JOIN users u ON pm.user_id = u.id
		WHERE pm.project_id = $1
		ORDER BY pm.id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project members failed: %w", err)
	}
	defer rows.Close()

	members := make([]ProjectMember, 0)
	for rows.Next() {
		var pm ProjectMember
		var addedBy sql.NullInt64
		var u auth.UserSummary
		if err := rows.Scan(
			&pm.ID, &pm.ProjectID, &pm.UserID, &pm.Role, &addedBy, &pm.CreatedAt,
			&u.ID, &u.Email, &u.FullName, &u.Role,
		); err != nil {
			return nil, err
		}
		if addedBy.Valid {
			ab := int(addedBy.Int64)
			pm.AddedBy = &ab
		}
		pm.User = &u
		members = append(members, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project members failed: %w", err)
	}
	return members, nil
}

func (r *PostgresRepository) RecordLog(ctx context.Context, log *ProjectLog) (*ProjectLog, error) {
	if log.Metadata == "" {
		log.Metadata = "{}"
	}
	now := time.Now().UTC()
	query := `
		INSERT INTO project_logs (project_id, actor_id, ticket_id, action, summary, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query, log.ProjectID, log.ActorID, log.TicketID, log.Action, log.Summary, log.Metadata, now).
		Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("record project log failed: %w", err)
	}
	return log, nil
}

func (r *PostgresRepository) ListLogs(ctx context.Context, projectID, limit, offset int) ([]ProjectLog, int, error) {
	countQuery := `SELECT COUNT(*) FROM project_logs WHERE project_id = $1`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, projectID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count project logs failed: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT pl.id, pl.project_id, pl.actor_id, pl.ticket_id, pl.action, pl.summary, pl.metadata::text, pl.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM project_logs pl
		LEFT JOIN users u ON pl.actor_id = u.id
		WHERE pl.project_id = $1
		ORDER BY pl.created_at DESC, pl.id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, projectID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list project logs failed: %w", err)
	}
	defer rows.Close()

	logs := make([]ProjectLog, 0)
	for rows.Next() {
		var pl ProjectLog
		var actorID, ticketID, uID sql.NullInt64
		var uEmail, uName, uRole sql.NullString

		if err := rows.Scan(
			&pl.ID, &pl.ProjectID, &actorID, &ticketID, &pl.Action, &pl.Summary, &pl.Metadata, &pl.CreatedAt,
			&uID, &uEmail, &uName, &uRole,
		); err != nil {
			return nil, 0, err
		}
		if actorID.Valid {
			aid := int(actorID.Int64)
			pl.ActorID = &aid
		}
		if ticketID.Valid {
			tid := int(ticketID.Int64)
			pl.TicketID = &tid
		}
		if uID.Valid {
			pl.Actor = &auth.UserSummary{
				ID:       int(uID.Int64),
				Email:    uEmail.String,
				FullName: uName.String,
				Role:     auth.Role(uRole.String),
			}
		}
		logs = append(logs, pl)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate project logs failed: %w", err)
	}
	return logs, total, nil
}
