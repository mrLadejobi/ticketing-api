package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateProject(ctx context.Context, p *Project) (*Project, error) {
	query := `
		INSERT INTO projects (team_id, name, prefix, description, issue_counter, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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

	err := r.db.QueryRowContext(ctx, query, p.TeamID, p.Name, p.Prefix, p.Description, p.IssueCounter, p.Status, p.CreatedAt, p.UpdatedAt).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			return nil, ErrDuplicatePrefix
		}
		return nil, fmt.Errorf("create project failed: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) GetProjectByID(ctx context.Context, id int) (*Project, error) {
	query := `SELECT id, team_id, name, prefix, description, issue_counter, status, created_at, updated_at FROM projects WHERE id = $1`
	var p Project
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.TeamID, &p.Name, &p.Prefix, &p.Description, &p.IssueCounter, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("get project by id failed: %w", err)
	}
	return &p, nil
}

func (r *PostgresRepository) GetProjectByPrefix(ctx context.Context, prefix string) (*Project, error) {
	query := `SELECT id, team_id, name, prefix, description, issue_counter, status, created_at, updated_at FROM projects WHERE UPPER(prefix) = UPPER($1)`
	var p Project
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(prefix)).Scan(
		&p.ID, &p.TeamID, &p.Name, &p.Prefix, &p.Description, &p.IssueCounter, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("get project by prefix failed: %w", err)
	}
	return &p, nil
}

func (r *PostgresRepository) ListProjects(ctx context.Context, teamID *int) ([]Project, error) {
	var query string
	var args []any
	if teamID != nil {
		query = `SELECT id, team_id, name, prefix, description, issue_counter, status, created_at, updated_at FROM projects WHERE team_id = $1 ORDER BY id ASC`
		args = append(args, *teamID)
	} else {
		query = `SELECT id, team_id, name, prefix, description, issue_counter, status, created_at, updated_at FROM projects ORDER BY id ASC`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects failed: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var p Project
		if err := rows.Scan(
			&p.ID, &p.TeamID, &p.Name, &p.Prefix, &p.Description, &p.IssueCounter, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
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

