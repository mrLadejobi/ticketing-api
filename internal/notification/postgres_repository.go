package notification

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

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) getExecutor(ctx context.Context) dbExecutor {
	if tx := database.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *PostgresRepository) Create(ctx context.Context, n *Notification) (*Notification, error) {
	query := `
		INSERT INTO notifications (user_id, actor_id, ticket_id, type, title, body, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	n.IsRead = false
	n.CreatedAt = now

	err := r.getExecutor(ctx).QueryRowContext(
		ctx, query,
		n.UserID, n.ActorID, n.TicketID, n.Type, n.Title, n.Body, n.IsRead, n.CreatedAt,
	).Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert notification failed: %w", err)
	}

	return n, nil
}

func (r *PostgresRepository) List(ctx context.Context, filter ListNotificationsFilter) ([]Notification, int, error) {
	where := "WHERE n.user_id = $1"
	args := []any{filter.UserID}
	argIdx := 2

	if filter.UnreadOnly {
		where += " AND n.is_read = FALSE"
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM notifications n %s", where)
	var total int
	if err := r.getExecutor(ctx).QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications failed: %w", err)
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	listQuery := fmt.Sprintf(`
		SELECT n.id, n.user_id, n.actor_id, n.ticket_id, n.type, n.title, n.body, n.is_read, n.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM notifications n
		LEFT JOIN users u ON n.actor_id = u.id
		%s
		ORDER BY n.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.getExecutor(ctx).QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications failed: %w", err)
	}
	defer rows.Close()

	var list []Notification
	for rows.Next() {
		var n Notification
		var actorID, ticketID sql.NullInt64
		var uID sql.NullInt64
		var uEmail, uName, uRole sql.NullString

		if err := rows.Scan(
			&n.ID, &n.UserID, &actorID, &ticketID, &n.Type, &n.Title, &n.Body, &n.IsRead, &n.CreatedAt,
			&uID, &uEmail, &uName, &uRole,
		); err != nil {
			return nil, 0, fmt.Errorf("scan notification failed: %w", err)
		}

		if actorID.Valid {
			aid := int(actorID.Int64)
			n.ActorID = &aid
		}
		if ticketID.Valid {
			tid := int(ticketID.Int64)
			n.TicketID = &tid
		}
		if uID.Valid {
			n.Actor = &auth.UserSummary{
				ID:       int(uID.Int64),
				Email:    uEmail.String,
				FullName: uName.String,
				Role:     auth.Role(uRole.String),
			}
		}

		list = append(list, n)
	}

	return list, total, nil
}

func (r *PostgresRepository) MarkAsRead(ctx context.Context, id, userID int) error {
	query := `UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`
	res, err := r.getExecutor(ctx).ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("mark notification as read failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) MarkAllAsRead(ctx context.Context, userID int) error {
	query := `UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`
	_, err := r.getExecutor(ctx).ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("mark all notifications as read failed: %w", err)
	}
	return nil
}

