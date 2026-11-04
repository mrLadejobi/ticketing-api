package ticket

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PostgresRepository implements Repository using PostgreSQL with context cancellation.
type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, t *Ticket) (*Ticket, error) {
	query := `
		INSERT INTO tickets (title, description, status, priority, assignee, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, status, created_at, updated_at
	`

	now := time.Now().UTC()
	t.Status = StatusTodo
	t.CreatedAt = now
	t.UpdatedAt = now

	err := r.db.QueryRowContext(
		ctx,
		query,
		t.Title,
		t.Description,
		t.Status,
		t.Priority,
		t.Assignee,
		t.CreatedAt,
		t.UpdatedAt,
	).Scan(&t.ID, &t.Status, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert ticket failed: %w", err)
	}

	return t, nil
}

func (r *PostgresRepository) List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error) {
	var whereClauses []string
	var args []any
	argID := 1

	if filter.Status != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argID))
		args = append(args, *filter.Status)
		argID++
	}

	if filter.Priority != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("priority = $%d", argID))
		args = append(args, *filter.Priority)
		argID++
	}

	if filter.Assignee != nil && *filter.Assignee != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("LOWER(assignee) = LOWER($%d)", argID))
		args = append(args, *filter.Assignee)
		argID++
	}

	if filter.Search != nil && *filter.Search != "" {
		likePattern := "%" + *filter.Search + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argID, argID))
		args = append(args, likePattern)
		argID++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 1. Get total count
	countQuery := "SELECT COUNT(*) FROM tickets" + whereSQL
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets failed: %w", err)
	}

	// 2. Fetch paginated records
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, title, description, status, priority, COALESCE(assignee, ''), created_at, updated_at
		FROM tickets%s
		ORDER BY id ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argID, argID+1)

	queryArgs := append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query tickets failed: %w", err)
	}
	defer rows.Close()

	tickets := make([]Ticket, 0)
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(
			&t.ID,
			&t.Title,
			&t.Description,
			&t.Status,
			&t.Priority,
			&t.Assignee,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan ticket failed: %w", err)
		}
		tickets = append(tickets, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("row iteration error: %w", err)
	}

	return tickets, total, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int) (*Ticket, error) {
	query := `
		SELECT id, title, description, status, priority, COALESCE(assignee, ''), created_at, updated_at
		FROM tickets
		WHERE id = $1
	`

	var t Ticket
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.Title,
		&t.Description,
		&t.Status,
		&t.Priority,
		&t.Assignee,
		&t.CreatedAt,
		&t.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query ticket by id failed: %w", err)
	}

	return &t, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.Assignee != nil {
		existing.Assignee = *req.Assignee
	}
	existing.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE tickets
		SET title = $1, description = $2, status = $3, priority = $4, assignee = $5, updated_at = $6
		WHERE id = $7
	`

	_, err = r.db.ExecContext(
		ctx,
		query,
		existing.Title,
		existing.Description,
		existing.Status,
		existing.Priority,
		existing.Assignee,
		existing.UpdatedAt,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("update ticket failed: %w", err)
	}

	return existing, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM tickets WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete ticket failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check affected rows failed: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
