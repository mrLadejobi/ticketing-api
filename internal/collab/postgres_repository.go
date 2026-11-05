package collab

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

func (r *PostgresRepository) CreateComment(ctx context.Context, c *Comment) (*Comment, error) {
	query := `
		INSERT INTO ticket_comments (ticket_id, author_id, body, mentions, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	// Format text array for Postgres literal: {"item1","item2"}
	mentionsArray := formatStringArray(c.Mentions)

	err := r.db.QueryRowContext(ctx, query, c.TicketID, c.AuthorID, c.Body, mentionsArray, c.CreatedAt, c.UpdatedAt).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create comment failed: %w", err)
	}

	return r.GetCommentByID(ctx, c.ID)
}

func (r *PostgresRepository) GetCommentByID(ctx context.Context, id int) (*Comment, error) {
	query := `
		SELECT c.id, c.ticket_id, c.author_id, c.body, array_to_string(c.mentions, ','), c.deleted_at, c.created_at, c.updated_at,
		       u.id, u.email, u.full_name, u.role
		FROM ticket_comments c
		JOIN users u ON c.author_id = u.id
		WHERE c.id = $1 AND c.deleted_at IS NULL
	`
	var c Comment
	var u auth.UserSummary
	var mentionsCSV sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.TicketID, &c.AuthorID, &c.Body, &mentionsCSV, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
		&u.ID, &u.Email, &u.FullName, &u.Role,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCommentNotFound
		}
		return nil, fmt.Errorf("get comment by id failed: %w", err)
	}

	c.Author = &u
	if mentionsCSV.Valid && mentionsCSV.String != "" {
		c.Mentions = strings.Split(mentionsCSV.String, ",")
	} else {
		c.Mentions = []string{}
	}

	return &c, nil
}

func (r *PostgresRepository) UpdateComment(ctx context.Context, id int, body string, mentions []string) (*Comment, error) {
	query := `
		UPDATE ticket_comments
		SET body = $1, mentions = $2, updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING id
	`
	now := time.Now().UTC()
	mentionsArray := formatStringArray(mentions)

	var cid int
	err := r.db.QueryRowContext(ctx, query, body, mentionsArray, now, id).Scan(&cid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCommentNotFound
		}
		return nil, fmt.Errorf("update comment failed: %w", err)
	}

	return r.GetCommentByID(ctx, id)
}

func (r *PostgresRepository) SoftDeleteComment(ctx context.Context, id int) error {
	query := `UPDATE ticket_comments SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("soft delete comment failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrCommentNotFound
	}
	return nil
}

func (r *PostgresRepository) ListCommentsByTicket(ctx context.Context, ticketID int) ([]Comment, error) {
	query := `
		SELECT c.id, c.ticket_id, c.author_id, c.body, array_to_string(c.mentions, ','), c.deleted_at, c.created_at, c.updated_at,
		       u.id, u.email, u.full_name, u.role
		FROM ticket_comments c
		JOIN users u ON c.author_id = u.id
		WHERE c.ticket_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list comments failed: %w", err)
	}
	defer rows.Close()

	comments := make([]Comment, 0)
	for rows.Next() {
		var c Comment
		var u auth.UserSummary
		var mentionsCSV sql.NullString

		if err := rows.Scan(
			&c.ID, &c.TicketID, &c.AuthorID, &c.Body, &mentionsCSV, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
			&u.ID, &u.Email, &u.FullName, &u.Role,
		); err != nil {
			return nil, err
		}
		c.Author = &u
		if mentionsCSV.Valid && mentionsCSV.String != "" {
			c.Mentions = strings.Split(mentionsCSV.String, ",")
		} else {
			c.Mentions = []string{}
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (r *PostgresRepository) AddWatcher(ctx context.Context, ticketID, userID int) (*Watcher, error) {
	query := `
		INSERT INTO ticket_watchers (ticket_id, user_id, created_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	var w Watcher
	w.TicketID = ticketID
	w.UserID = userID
	err := r.db.QueryRowContext(ctx, query, ticketID, userID, now).Scan(&w.ID, &w.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "23505") {
			return nil, ErrWatcherDuplicate
		}
		return nil, fmt.Errorf("add watcher failed: %w", err)
	}

	// Fetch user summary
	userQuery := `SELECT id, email, full_name, role FROM users WHERE id = $1`
	var u auth.UserSummary
	if err := r.db.QueryRowContext(ctx, userQuery, userID).Scan(&u.ID, &u.Email, &u.FullName, &u.Role); err == nil {
		w.User = &u
	}

	return &w, nil
}

func (r *PostgresRepository) RemoveWatcher(ctx context.Context, ticketID, userID int) error {
	query := `DELETE FROM ticket_watchers WHERE ticket_id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, query, ticketID, userID)
	if err != nil {
		return fmt.Errorf("remove watcher failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrWatcherNotFound
	}
	return nil
}

func (r *PostgresRepository) ListWatchersByTicket(ctx context.Context, ticketID int) ([]Watcher, error) {
	query := `
		SELECT w.id, w.ticket_id, w.user_id, w.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM ticket_watchers w
		JOIN users u ON w.user_id = u.id
		WHERE w.ticket_id = $1
		ORDER BY w.id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list watchers failed: %w", err)
	}
	defer rows.Close()

	watchers := make([]Watcher, 0)
	for rows.Next() {
		var w Watcher
		var u auth.UserSummary
		if err := rows.Scan(
			&w.ID, &w.TicketID, &w.UserID, &w.CreatedAt,
			&u.ID, &u.Email, &u.FullName, &u.Role,
		); err != nil {
			return nil, err
		}
		w.User = &u
		watchers = append(watchers, w)
	}
	return watchers, nil
}

func (r *PostgresRepository) CreateAttachment(ctx context.Context, a *Attachment) (*Attachment, error) {
	query := `
		INSERT INTO ticket_attachments (ticket_id, uploader_id, file_name, file_size, content_type, file_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(ctx, query, a.TicketID, a.UploaderID, a.FileName, a.FileSize, a.ContentType, a.FileURL, now).
		Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create attachment failed: %w", err)
	}

	userQuery := `SELECT id, email, full_name, role FROM users WHERE id = $1`
	var u auth.UserSummary
	if err := r.db.QueryRowContext(ctx, userQuery, a.UploaderID).Scan(&u.ID, &u.Email, &u.FullName, &u.Role); err == nil {
		a.Uploader = &u
	}

	return a, nil
}

func (r *PostgresRepository) ListAttachmentsByTicket(ctx context.Context, ticketID int) ([]Attachment, error) {
	query := `
		SELECT a.id, a.ticket_id, a.uploader_id, a.file_name, a.file_size, a.content_type, a.file_url, a.created_at,
		       u.id, u.email, u.full_name, u.role
		FROM ticket_attachments a
		JOIN users u ON a.uploader_id = u.id
		WHERE a.ticket_id = $1
		ORDER BY a.id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list attachments failed: %w", err)
	}
	defer rows.Close()

	attachments := make([]Attachment, 0)
	for rows.Next() {
		var a Attachment
		var u auth.UserSummary
		if err := rows.Scan(
			&a.ID, &a.TicketID, &a.UploaderID, &a.FileName, &a.FileSize, &a.ContentType, &a.FileURL, &a.CreatedAt,
			&u.ID, &u.Email, &u.FullName, &u.Role,
		); err != nil {
			return nil, err
		}
		a.Uploader = &u
		attachments = append(attachments, a)
	}
	return attachments, nil
}

func formatStringArray(slice []string) string {
	if len(slice) == 0 {
		return "{}"
	}
	escaped := make([]string, len(slice))
	for i, s := range slice {
		escaped[i] = `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "{" + strings.Join(escaped, ",") + "}"
}

