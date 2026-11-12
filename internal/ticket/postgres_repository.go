package ticket

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

func (r *PostgresRepository) Create(ctx context.Context, t *Ticket) (*Ticket, error) {
	query := `
		INSERT INTO tickets (
			project_id, parent_id, cycle_id, key, ticket_type, title, description, work_summary, status, priority, progress_percent, due_date, reporter_id, assignee_id, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 1, $15, $16)
		RETURNING id, created_at, updated_at
	`
	now := time.Now().UTC()
	t.Version = 1
	if t.Status == "" {
		t.Status = StatusTodo
	}
	if t.TicketType == "" {
		t.TicketType = TypeTask
	}
	t.CreatedAt = now
	t.UpdatedAt = now

	err := r.getExecutor(ctx).QueryRowContext(
		ctx, query,
		t.ProjectID, t.ParentID, t.CycleID, t.Key, t.TicketType, t.Title, t.Description, t.WorkSummary, t.Status, t.Priority, t.ProgressPercent, t.DueDate, t.ReporterID, t.AssigneeID, t.CreatedAt, t.UpdatedAt,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert ticket failed: %w", err)
	}

	return r.GetByIDOrKey(ctx, strconv.Itoa(t.ID))
}

func (r *PostgresRepository) List(ctx context.Context, filter ListTicketsFilter) ([]Ticket, int, error) {
	var whereClauses []string
	var args []any
	argID := 1

	if !filter.IncludeDeleted {
		whereClauses = append(whereClauses, "t.deleted_at IS NULL")
	}

	if filter.ProjectID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.project_id = $%d", argID))
		args = append(args, *filter.ProjectID)
		argID++
	}

	if filter.CycleID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.cycle_id = $%d", argID))
		args = append(args, *filter.CycleID)
		argID++
	}

	if filter.Status != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.status = $%d", argID))
		args = append(args, *filter.Status)
		argID++
	}

	if filter.TicketType != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.ticket_type = $%d", argID))
		args = append(args, *filter.TicketType)
		argID++
	}

	if filter.Priority != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.priority = $%d", argID))
		args = append(args, *filter.Priority)
		argID++
	}

	if filter.AssigneeID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.assignee_id = $%d", argID))
		args = append(args, *filter.AssigneeID)
		argID++
	}

	if filter.ReporterID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.reporter_id = $%d", argID))
		args = append(args, *filter.ReporterID)
		argID++
	}

	if filter.Search != nil && *filter.Search != "" {
		likePattern := "%" + *filter.Search + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(t.title ILIKE $%d OR t.description ILIKE $%d OR t.key ILIKE $%d)", argID, argID, argID))
		args = append(args, likePattern)
		argID++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM tickets t" + whereSQL
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets failed: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT t.id, t.project_id, t.parent_id, t.cycle_id, t.key, t.ticket_type, t.title, t.description, t.work_summary, t.status, t.priority, t.progress_percent, t.due_date,
		       t.reporter_id, t.assignee_id, t.version, t.deleted_at, t.created_at, t.updated_at,
		       u_rep.id, u_rep.email, u_rep.full_name, u_rep.role,
		       u_ass.id, u_ass.email, u_ass.full_name, u_ass.role
		FROM tickets t
		LEFT JOIN users u_rep ON t.reporter_id = u_rep.id
		LEFT JOIN users u_ass ON t.assignee_id = u_ass.id
		%s
		ORDER BY t.id ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argID, argID+1)

	queryArgs := append(args, limit, offset)
	rows, err := r.getExecutor(ctx).QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query tickets failed: %w", err)
	}
	defer rows.Close()

	tickets := make([]Ticket, 0)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, 0, err
		}
		tickets = append(tickets, *t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tickets failed: %w", err)
	}

	return tickets, total, nil
}

func (r *PostgresRepository) GetByIDOrKey(ctx context.Context, idOrKey string) (*Ticket, error) {
	var whereClause string
	var arg any

	if id, err := strconv.Atoi(idOrKey); err == nil {
		whereClause = "t.id = $1"
		arg = id
	} else {
		whereClause = "UPPER(t.key) = UPPER($1)"
		arg = strings.TrimSpace(idOrKey)
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.project_id, t.parent_id, t.cycle_id, t.key, t.ticket_type, t.title, t.description, t.work_summary, t.status, t.priority, t.progress_percent, t.due_date,
		       t.reporter_id, t.assignee_id, t.version, t.deleted_at, t.created_at, t.updated_at,
		       u_rep.id, u_rep.email, u_rep.full_name, u_rep.role,
		       u_ass.id, u_ass.email, u_ass.full_name, u_ass.role
		FROM tickets t
		LEFT JOIN users u_rep ON t.reporter_id = u_rep.id
		LEFT JOIN users u_ass ON t.assignee_id = u_ass.id
		WHERE %s
	`, whereClause)

	row := r.getExecutor(ctx).QueryRowContext(ctx, query, arg)
	t, err := scanTicketRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get ticket by id or key failed: %w", err)
	}
	return t, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id int, req UpdateTicketRequest) (*Ticket, error) {
	current, err := r.GetByIDOrKey(ctx, strconv.Itoa(id))
	if err != nil {
		return nil, err
	}
	if current.DeletedAt != nil {
		return nil, ErrNotFound
	}

	if req.Version != nil && *req.Version != current.Version {
		return nil, ErrOptimisticLockConflict
	}

	if req.ParentID != nil {
		current.ParentID = req.ParentID
	}
	if req.CycleID != nil {
		current.CycleID = req.CycleID
	}
	if req.TicketType != nil {
		current.TicketType = *req.TicketType
	}
	if req.Title != nil {
		current.Title = *req.Title
	}
	if req.Description != nil {
		current.Description = *req.Description
	}
	if req.WorkSummary != nil {
		current.WorkSummary = *req.WorkSummary
	}
	if req.Status != nil {
		current.Status = *req.Status
	}
	if req.Priority != nil {
		current.Priority = *req.Priority
	}
	if req.ProgressPercent != nil {
		current.ProgressPercent = req.ProgressPercent
	}
	if req.DueDate != nil {
		current.DueDate = req.DueDate
	}
	if req.AssigneeID != nil {
		current.AssigneeID = req.AssigneeID
	}
	current.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE tickets
		SET parent_id = $1, cycle_id = $2, ticket_type = $3, title = $4, description = $5, work_summary = $6, status = $7, priority = $8, progress_percent = $9, due_date = $10, assignee_id = $11,
		    version = version + 1, updated_at = $12
		WHERE id = $13 AND version = $14 AND deleted_at IS NULL
	`
	res, err := r.getExecutor(ctx).ExecContext(ctx, query, current.ParentID, current.CycleID, current.TicketType, current.Title, current.Description, current.WorkSummary, current.Status, current.Priority, current.ProgressPercent, current.DueDate, current.AssigneeID, current.UpdatedAt, id, current.Version)
	if err != nil {
		return nil, fmt.Errorf("update ticket failed: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, ErrOptimisticLockConflict
	}

	return r.GetByIDOrKey(ctx, strconv.Itoa(id))
}

func (r *PostgresRepository) SoftDelete(ctx context.Context, id int) error {
	query := `UPDATE tickets SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("soft delete ticket failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Restore(ctx context.Context, id int) (*Ticket, error) {
	query := `UPDATE tickets SET deleted_at = NULL, updated_at = $1 WHERE id = $2 RETURNING id`
	now := time.Now().UTC()
	var tid int
	err := r.db.QueryRowContext(ctx, query, now, id).Scan(&tid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("restore ticket failed: %w", err)
	}
	return r.GetByIDOrKey(ctx, strconv.Itoa(id))
}

func (r *PostgresRepository) ListByProject(ctx context.Context, projectID int) ([]Ticket, error) {
	filter := ListTicketsFilter{
		ProjectID:      &projectID,
		IncludeDeleted: false,
		Limit:          1000,
	}
	tickets, _, err := r.List(ctx, filter)
	return tickets, err
}

func (r *PostgresRepository) ListSubtasks(ctx context.Context, parentID int) ([]Ticket, error) {
	query := `
		SELECT t.id, t.project_id, t.parent_id, t.cycle_id, t.key, t.ticket_type, t.title, t.description, t.work_summary, t.status, t.priority, t.progress_percent, t.due_date,
		       t.reporter_id, t.assignee_id, t.version, t.deleted_at, t.created_at, t.updated_at,
		       u_rep.id, u_rep.email, u_rep.full_name, u_rep.role,
		       u_ass.id, u_ass.email, u_ass.full_name, u_ass.role
		FROM tickets t
		LEFT JOIN users u_rep ON t.reporter_id = u_rep.id
		LEFT JOIN users u_ass ON t.assignee_id = u_ass.id
		WHERE t.parent_id = $1 AND t.deleted_at IS NULL
		ORDER BY t.id ASC
	`
	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("list subtasks failed: %w", err)
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtasks failed: %w", err)
	}
	return tickets, nil
}

func (r *PostgresRepository) ListByCycle(ctx context.Context, cycleID int) ([]Ticket, error) {
	query := `
		SELECT t.id, t.project_id, t.parent_id, t.cycle_id, t.key, t.ticket_type, t.title, t.description, t.work_summary, t.status, t.priority, t.progress_percent, t.due_date,
		       t.reporter_id, t.assignee_id, t.version, t.deleted_at, t.created_at, t.updated_at,
		       u_rep.id, u_rep.email, u_rep.full_name, u_rep.role,
		       u_ass.id, u_ass.email, u_ass.full_name, u_ass.role
		FROM tickets t
		LEFT JOIN users u_rep ON t.reporter_id = u_rep.id
		LEFT JOIN users u_ass ON t.assignee_id = u_ass.id
		WHERE t.cycle_id = $1 AND t.deleted_at IS NULL
		ORDER BY t.id ASC
	`
	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, cycleID)
	if err != nil {
		return nil, fmt.Errorf("list by cycle failed: %w", err)
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtasks failed: %w", err)
	}
	return tickets, nil
}

func (r *PostgresRepository) BulkUpdate(ctx context.Context, req BulkUpdateTicketsRequest) ([]Ticket, error) {
	if len(req.TicketIDs) == 0 {
		return []Ticket{}, nil
	}

	exec := r.getExecutor(ctx)
	var tx *sql.Tx
	var isOwner bool

	if existingTx := database.TxFromContext(ctx); existingTx != nil {
		tx = existingTx
	} else {
		var err error
		tx, err = r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to begin bulk update transaction: %w", err)
		}
		isOwner = true
		defer func() {
			if isOwner && tx != nil {
				_ = tx.Rollback()
			}
		}()
		exec = tx
	}

	now := time.Now().UTC()
	for _, id := range req.TicketIDs {
		var setClauses []string
		var args []any
		argIdx := 1

		if req.Status != nil {
			setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
			args = append(args, *req.Status)
			argIdx++
		}
		if req.Priority != nil {
			setClauses = append(setClauses, fmt.Sprintf("priority = $%d", argIdx))
			args = append(args, *req.Priority)
			argIdx++
		}
		if req.AssigneeID != nil {
			setClauses = append(setClauses, fmt.Sprintf("assignee_id = $%d", argIdx))
			args = append(args, *req.AssigneeID)
			argIdx++
		}
		if req.CycleID != nil {
			setClauses = append(setClauses, fmt.Sprintf("cycle_id = $%d", argIdx))
			args = append(args, *req.CycleID)
			argIdx++
		}

		if len(setClauses) == 0 {
			continue
		}

		setClauses = append(setClauses, fmt.Sprintf("version = version + 1, updated_at = $%d", argIdx))
		args = append(args, now)
		argIdx++

		args = append(args, id)
		query := fmt.Sprintf("UPDATE tickets SET %s WHERE id = $%d AND deleted_at IS NULL", strings.Join(setClauses, ", "), argIdx)

		if _, err := exec.ExecContext(ctx, query, args...); err != nil {
			return nil, fmt.Errorf("failed to bulk update ticket %d: %w", id, err)
		}
	}

	if isOwner {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit bulk update transaction: %w", err)
		}
		tx = nil
	}

	var updated []Ticket
	for _, id := range req.TicketIDs {
		if t, err := r.GetByIDOrKey(ctx, strconv.Itoa(id)); err == nil {
			updated = append(updated, *t)
		}
	}
	return updated, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTicketScanner(scanner rowScanner) (*Ticket, error) {
	var t Ticket
	var parentID, cycleID, progress, uRepID, uAssID sql.NullInt64
	var dueDate sql.NullTime
	var uRepEmail, uRepName, uRepRole sql.NullString
	var uAssEmail, uAssName, uAssRole sql.NullString

	if err := scanner.Scan(
		&t.ID, &t.ProjectID, &parentID, &cycleID, &t.Key, &t.TicketType, &t.Title, &t.Description, &t.WorkSummary, &t.Status, &t.Priority, &progress, &dueDate,
		&t.ReporterID, &t.AssigneeID, &t.Version, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt,
		&uRepID, &uRepEmail, &uRepName, &uRepRole,
		&uAssID, &uAssEmail, &uAssName, &uAssRole,
	); err != nil {
		return nil, err
	}

	if parentID.Valid {
		pid := int(parentID.Int64)
		t.ParentID = &pid
	}
	if cycleID.Valid {
		cid := int(cycleID.Int64)
		t.CycleID = &cid
	}
	if progress.Valid {
		p := int(progress.Int64)
		t.ProgressPercent = &p
	}
	if dueDate.Valid {
		d := dueDate.Time
		t.DueDate = &d
	}

	if uRepID.Valid {
		t.Reporter = &auth.UserSummary{
			ID:       int(uRepID.Int64),
			Email:    uRepEmail.String,
			FullName: uRepName.String,
			Role:     auth.Role(uRepRole.String),
		}
	}
	if uAssID.Valid {
		t.Assignee = &auth.UserSummary{
			ID:       int(uAssID.Int64),
			Email:    uAssEmail.String,
			FullName: uAssName.String,
			Role:     auth.Role(uAssRole.String),
		}
	}

	return &t, nil
}

func scanTicket(rows *sql.Rows) (*Ticket, error) {
	t, err := scanTicketScanner(rows)
	if err != nil {
		return nil, fmt.Errorf("scan ticket failed: %w", err)
	}
	return t, nil
}

func scanTicketRow(row *sql.Row) (*Ticket, error) {
	return scanTicketScanner(row)
}
