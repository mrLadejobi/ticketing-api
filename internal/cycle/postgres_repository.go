package cycle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, c *Cycle) (*Cycle, error) {
	query := `
		INSERT INTO project_cycles (
			project_id, name, goal, start_date, end_date, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`
	now := time.Now().UTC()
	if c.Status == "" {
		c.Status = StatusPlanned
	}

	err := r.db.QueryRowContext(
		ctx, query,
		c.ProjectID, c.Name, c.Goal, c.StartDate, c.EndDate, c.Status, now, now,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create cycle: %w", err)
	}

	return c, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int) (*Cycle, error) {
	query := `
		SELECT id, project_id, name, goal, start_date, end_date, status, created_at, updated_at
		FROM project_cycles
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var c Cycle
	if err := row.Scan(
		&c.ID, &c.ProjectID, &c.Name, &c.Goal, &c.StartDate, &c.EndDate, &c.Status, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get cycle: %w", err)
	}

	return &c, nil
}

func (r *PostgresRepository) ListByProjectID(ctx context.Context, projectID int) ([]Cycle, error) {
	query := `
		SELECT id, project_id, name, goal, start_date, end_date, status, created_at, updated_at
		FROM project_cycles
		WHERE project_id = $1
		ORDER BY start_date ASC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project cycles: %w", err)
	}
	defer rows.Close()

	var cycles []Cycle
	for rows.Next() {
		var c Cycle
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.Name, &c.Goal, &c.StartDate, &c.EndDate, &c.Status, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan cycle: %w", err)
		}
		cycles = append(cycles, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cycles failed: %w", err)
	}

	return cycles, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id int, c *Cycle) (*Cycle, error) {
	query := `
		UPDATE project_cycles
		SET name = $1, goal = $2, start_date = $3, end_date = $4, status = $5, updated_at = $6
		WHERE id = $7
		RETURNING updated_at
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(
		ctx, query,
		c.Name, c.Goal, c.StartDate, c.EndDate, c.Status, now, id,
	).Scan(&c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update cycle: %w", err)
	}

	c.ID = id
	return c, nil
}

