package team

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

func (r *PostgresRepository) CreateTeam(ctx context.Context, t *Team) (*Team, error) {
	query := `
		INSERT INTO teams (name, slug, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	now := time.Now().UTC()
	t.Slug = strings.ToLower(strings.TrimSpace(t.Slug))
	t.CreatedAt = now
	t.UpdatedAt = now

	err := r.db.QueryRowContext(ctx, query, t.Name, t.Slug, t.Description, t.CreatedAt, t.UpdatedAt).
		Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("create team failed: %w", err)
	}
	return t, nil
}

func (r *PostgresRepository) GetTeamByID(ctx context.Context, id int) (*Team, error) {
	query := `SELECT id, name, slug, description, created_at, updated_at FROM teams WHERE id = $1`
	var t Team
	err := r.db.QueryRowContext(ctx, query, id).Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("get team by id failed: %w", err)
	}
	return &t, nil
}

func (r *PostgresRepository) GetTeamBySlug(ctx context.Context, slug string) (*Team, error) {
	query := `SELECT id, name, slug, description, created_at, updated_at FROM teams WHERE LOWER(slug) = LOWER($1)`
	var t Team
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(slug)).Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("get team by slug failed: %w", err)
	}
	return &t, nil
}

func (r *PostgresRepository) ListTeams(ctx context.Context) ([]Team, error) {
	query := `SELECT id, name, slug, description, created_at, updated_at FROM teams ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list teams failed: %w", err)
	}
	defer rows.Close()

	teams := make([]Team, 0)
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate teams failed: %w", err)
	}
	return teams, nil
}

func (r *PostgresRepository) AddMember(ctx context.Context, tm *TeamMember) (*TeamMember, error) {
	query := `
		INSERT INTO team_members (team_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query, tm.TeamID, tm.UserID, tm.Role, now).Scan(&tm.ID, &tm.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			return nil, ErrMemberDuplicate
		}
		return nil, fmt.Errorf("add member failed: %w", err)
	}

	// Fetch user details for summary
	userQuery := `SELECT id, email, full_name, role FROM users WHERE id = $1`
	var u auth.UserSummary
	if err := r.db.QueryRowContext(ctx, userQuery, tm.UserID).Scan(&u.ID, &u.Email, &u.FullName, &u.Role); err == nil {
		tm.User = &u
	}

	return tm, nil
}

func (r *PostgresRepository) RemoveMember(ctx context.Context, teamID, userID int) error {
	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("remove member failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *PostgresRepository) ListMembers(ctx context.Context, teamID int) ([]TeamMember, error) {
	query := `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = $1
		ORDER BY tm.id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team members failed: %w", err)
	}
	defer rows.Close()

	members := make([]TeamMember, 0)
	for rows.Next() {
		var tm TeamMember
		var u auth.UserSummary
		if err := rows.Scan(
			&tm.ID, &tm.TeamID, &tm.UserID, &tm.Role, &tm.CreatedAt,
			&u.ID, &u.Email, &u.FullName, &u.Role,
		); err != nil {
			return nil, err
		}
		tm.User = &u
		members = append(members, tm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members failed: %w", err)
	}
	return members, nil
}

