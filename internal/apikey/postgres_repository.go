package apikey

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

func (r *PostgresRepository) Create(ctx context.Context, key *APIKey) (*APIKey, error) {
	query := `
		INSERT INTO api_keys (user_id, name, prefix, key_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	now := time.Now().UTC()
	err := r.db.QueryRowContext(
		ctx, query,
		key.UserID, key.Name, key.Prefix, key.KeyHash, key.ExpiresAt, now,
	).Scan(&key.ID, &key.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert api key: %w", err)
	}

	return key, nil
}

func (r *PostgresRepository) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	query := `
		SELECT id, user_id, name, prefix, key_hash, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE key_hash = $1
	`
	row := r.db.QueryRowContext(ctx, query, hash)

	var key APIKey
	if err := row.Scan(
		&key.ID, &key.UserID, &key.Name, &key.Prefix, &key.KeyHash, &key.LastUsedAt, &key.ExpiresAt, &key.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	return &key, nil
}

func (r *PostgresRepository) UpdateLastUsed(ctx context.Context, id int) error {
	query := `UPDATE api_keys SET last_used_at = $1 WHERE id = $2`
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, query, now, id)
	return err
}

func (r *PostgresRepository) ListByUserID(ctx context.Context, userID int) ([]APIKey, error) {
	query := `
		SELECT id, user_id, name, prefix, key_hash, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY id DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(
			&key.ID, &key.UserID, &key.Name, &key.Prefix, &key.KeyHash, &key.LastUsedAt, &key.ExpiresAt, &key.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan api key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id int, userID int) error {
	query := `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete api key: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

