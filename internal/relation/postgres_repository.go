package relation

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

func (r *PostgresRepository) Create(ctx context.Context, rel *TicketRelation) (*TicketRelation, error) {
	if rel.SourceTicketID == rel.TargetTicketID {
		return nil, ErrSelfRelation
	}

	query := `
		INSERT INTO ticket_relations (source_ticket_id, target_ticket_id, relation_type, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query, rel.SourceTicketID, rel.TargetTicketID, rel.RelationType, now).Scan(&rel.ID, &rel.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "uq_ticket_relation") {
			return nil, ErrRelationExists
		}
		if strings.Contains(err.Error(), "chk_no_self_relation") {
			return nil, ErrSelfRelation
		}
		return nil, fmt.Errorf("failed to create ticket relation: %w", err)
	}

	return r.GetByID(ctx, rel.ID)
}

func (r *PostgresRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM ticket_relations WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete ticket relation: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int) (*TicketRelation, error) {
	query := `
		SELECT r.id, r.source_ticket_id, r.target_ticket_id, r.relation_type, r.created_at,
		       ts.id, ts.key, ts.title, ts.status, ts.priority,
		       tt.id, tt.key, tt.title, tt.status, tt.priority
		FROM ticket_relations r
		JOIN tickets ts ON r.source_ticket_id = ts.id
		JOIN tickets tt ON r.target_ticket_id = tt.id
		WHERE r.id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var rel TicketRelation
	var src TicketSummary
	var tgt TicketSummary

	err := row.Scan(
		&rel.ID, &rel.SourceTicketID, &rel.TargetTicketID, &rel.RelationType, &rel.CreatedAt,
		&src.ID, &src.Key, &src.Title, &src.Status, &src.Priority,
		&tgt.ID, &tgt.Key, &tgt.Title, &tgt.Status, &tgt.Priority,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get ticket relation: %w", err)
	}

	rel.SourceTicket = &src
	rel.TargetTicket = &tgt
	return &rel, nil
}

func (r *PostgresRepository) ListByTicketID(ctx context.Context, ticketID int) ([]TicketRelation, error) {
	query := `
		SELECT r.id, r.source_ticket_id, r.target_ticket_id, r.relation_type, r.created_at,
		       ts.id, ts.key, ts.title, ts.status, ts.priority,
		       tt.id, tt.key, tt.title, tt.status, tt.priority
		FROM ticket_relations r
		JOIN tickets ts ON r.source_ticket_id = ts.id
		JOIN tickets tt ON r.target_ticket_id = tt.id
		WHERE r.source_ticket_id = $1 OR r.target_ticket_id = $1
		ORDER BY r.id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ticket relations: %w", err)
	}
	defer rows.Close()

	var result []TicketRelation
	for rows.Next() {
		var rel TicketRelation
		var src TicketSummary
		var tgt TicketSummary

		if err := rows.Scan(
			&rel.ID, &rel.SourceTicketID, &rel.TargetTicketID, &rel.RelationType, &rel.CreatedAt,
			&src.ID, &src.Key, &src.Title, &src.Status, &src.Priority,
			&tgt.ID, &tgt.Key, &tgt.Title, &tgt.Status, &tgt.Priority,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ticket relation: %w", err)
		}

		rel.SourceTicket = &src
		rel.TargetTicket = &tgt
		result = append(result, rel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relations failed: %w", err)
	}

	return result, nil
}

