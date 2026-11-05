package auth

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

func (r *PostgresRepository) CreateUser(ctx context.Context, u *User) (*User, error) {
	query := `
		INSERT INTO users (email, password_hash, full_name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	now := time.Now().UTC()
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	u.CreatedAt = now
	u.UpdatedAt = now

	err := r.db.QueryRowContext(
		ctx,
		query,
		u.Email,
		u.PasswordHash,
		u.FullName,
		u.Role,
		u.CreatedAt,
		u.UpdatedAt,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return nil, ErrDuplicateEmail
		}
		return nil, fmt.Errorf("create user query failed: %w", err)
	}

	return u, nil
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	var u User
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(email)).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email failed: %w", err)
	}

	return &u, nil
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, id int) (*User, error) {
	query := `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var u User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id failed: %w", err)
	}

	return &u, nil
}

func (r *PostgresRepository) UpdateUserRole(ctx context.Context, userID int, role Role) (*User, error) {
	query := `
		UPDATE users
		SET role = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, email, password_hash, full_name, role, created_at, updated_at
	`
	now := time.Now().UTC()
	var u User
	err := r.db.QueryRowContext(ctx, query, role, now, userID).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("update user role failed: %w", err)
	}

	return &u, nil
}

func (r *PostgresRepository) CountUsers(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users failed: %w", err)
	}
	return count, nil
}

func (r *PostgresRepository) ListUsers(ctx context.Context) ([]User, error) {
	query := `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users
		ORDER BY id ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users failed: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID,
			&u.Email,
			&u.PasswordHash,
			&u.FullName,
			&u.Role,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user failed: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}
