package label

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

func (r *PostgresRepository) CreateProjectLabel(ctx context.Context, l *Label) (*Label, error) {
	query := `
		INSERT INTO project_labels (project_id, name, color_hex, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query, l.ProjectID, l.Name, l.ColorHex, now).Scan(&l.ID, &l.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "uq_project_label_name") {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to create project label: %w", err)
	}
	return l, nil
}

func (r *PostgresRepository) GetProjectLabelByID(ctx context.Context, id int) (*Label, error) {
	query := `SELECT id, project_id, name, color_hex, created_at FROM project_labels WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var l Label
	if err := row.Scan(&l.ID, &l.ProjectID, &l.Name, &l.ColorHex, &l.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get project label: %w", err)
	}
	return &l, nil
}

func (r *PostgresRepository) ListProjectLabels(ctx context.Context, projectID int) ([]Label, error) {
	query := `SELECT id, project_id, name, color_hex, created_at FROM project_labels WHERE project_id = $1 ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project labels: %w", err)
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.ColorHex, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan project label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, nil
}

func (r *PostgresRepository) AddTicketLabel(ctx context.Context, ticketID int, labelID int) error {
	query := `
		INSERT INTO ticket_labels (ticket_id, label_id, created_at)
		VALUES ($1, $2, $3)
	`
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, query, ticketID, labelID, now)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "uq_ticket_label") {
			return ErrAlreadyApplied
		}
		if strings.Contains(err.Error(), "foreign key") {
			return ErrNotFound
		}
		return fmt.Errorf("failed to attach label to ticket: %w", err)
	}
	return nil
}

func (r *PostgresRepository) RemoveTicketLabel(ctx context.Context, ticketID int, labelID int) error {
	query := `DELETE FROM ticket_labels WHERE ticket_id = $1 AND label_id = $2`
	res, err := r.db.ExecContext(ctx, query, ticketID, labelID)
	if err != nil {
		return fmt.Errorf("failed to remove ticket label: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListTicketLabels(ctx context.Context, ticketID int) ([]Label, error) {
	query := `
		SELECT l.id, l.project_id, l.name, l.color_hex, l.created_at
		FROM project_labels l
		JOIN ticket_labels tl ON l.id = tl.label_id
		WHERE tl.ticket_id = $1
		ORDER BY l.id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ticket labels: %w", err)
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.ColorHex, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan ticket label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, nil
}
