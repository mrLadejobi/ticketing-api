package activity

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ticketing-api/internal/auth"
	"ticketing-api/internal/database"
)

type dbExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type PostgresRepository struct {
	db *sql.DB
}

func (r *PostgresRepository) getExecutor(ctx context.Context) dbExecutor {
	if tx := database.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.db
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) RecordTicketActivity(ctx context.Context, act *TicketActivity) (*TicketActivity, error) {
	query := `
		INSERT INTO ticket_activities (ticket_id, actor_id, field, old_value, new_value, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	err := r.getExecutor(ctx).QueryRowContext(
		ctx, query,
		act.TicketID, act.ActorID, act.Field, act.OldValue, act.NewValue, now,
	).Scan(&act.ID, &act.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("record ticket activity failed: %w", err)
	}

	return act, nil
}

func (r *PostgresRepository) ListTicketActivities(ctx context.Context, ticketID int, limit, offset int) ([]TicketActivity, int, error) {
	countQuery := `SELECT COUNT(*) FROM ticket_activities WHERE ticket_id = $1`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, ticketID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ticket activities failed: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT a.id, a.ticket_id, a.actor_id, a.field, a.old_value, a.new_value, a.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM ticket_activities a
		LEFT JOIN users u ON a.actor_id = u.id
		WHERE a.ticket_id = $1
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, ticketID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list ticket activities failed: %w", err)
	}
	defer rows.Close()

	var list []TicketActivity
	for rows.Next() {
		var act TicketActivity
		var uID sql.NullInt64
		var uEmail, uName, uRole sql.NullString

		if err := rows.Scan(
			&act.ID, &act.TicketID, &act.ActorID, &act.Field, &act.OldValue, &act.NewValue, &act.CreatedAt,
			&uID, &uEmail, &uName, &uRole,
		); err != nil {
			return nil, 0, fmt.Errorf("scan ticket activity failed: %w", err)
		}

		if uID.Valid {
			act.Actor = &auth.UserSummary{
				ID:       int(uID.Int64),
				Email:    uEmail.String,
				FullName: uName.String,
				Role:     auth.Role(uRole.String),
			}
		}
		list = append(list, act)
	}

	return list, total, nil
}

func (r *PostgresRepository) RecordAuditLog(ctx context.Context, log *AuditLog) (*AuditLog, error) {
	query := `
		INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, payload, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(
		ctx, query,
		log.ActorID, log.Action, log.ResourceType, log.ResourceID, log.Payload, log.IPAddress, now,
	).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("record audit log failed: %w", err)
	}

	return log, nil
}

func (r *PostgresRepository) ListAuditLogs(ctx context.Context, limit, offset int) ([]AuditLog, int, error) {
	countQuery := `SELECT COUNT(*) FROM audit_logs`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs failed: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT l.id, l.actor_id, l.action, l.resource_type, l.resource_id, l.payload, l.ip_address, l.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM audit_logs l
		LEFT JOIN users u ON l.actor_id = u.id
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs failed: %w", err)
	}
	defer rows.Close()

	var list []AuditLog
	for rows.Next() {
		var l AuditLog
		var uID sql.NullInt64
		var uEmail, uName, uRole sql.NullString

		if err := rows.Scan(
			&l.ID, &l.ActorID, &l.Action, &l.ResourceType, &l.ResourceID, &l.Payload, &l.IPAddress, &l.CreatedAt,
			&uID, &uEmail, &uName, &uRole,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit log failed: %w", err)
		}

		if uID.Valid {
			l.Actor = &auth.UserSummary{
				ID:       int(uID.Int64),
				Email:    uEmail.String,
				FullName: uName.String,
				Role:     auth.Role(uRole.String),
			}
		}
		list = append(list, l)
	}

	return list, total, nil
}

