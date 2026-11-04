package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"ticketing-api/internal/config"
)

// Connect initializes a connection pool to PostgreSQL and verifies it with a ping.
func Connect(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(25)                 // Max number of active open connections
	db.SetMaxIdleConns(25)                 // Max number of connections in the idle pool
	db.SetConnMaxLifetime(5 * time.Minute) // Maximum time a connection may be reused
	db.SetConnMaxIdleTime(1 * time.Minute) // Maximum time a connection may remain idle

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return db, nil
}

// Migrate ensures the required tables and indexes exist.
func Migrate(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS tickets (
		id SERIAL PRIMARY KEY,
		title VARCHAR(100) NOT NULL,
		description TEXT NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'TODO',
		priority VARCHAR(20) NOT NULL DEFAULT 'MEDIUM',
		assignee VARCHAR(100),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
	CREATE INDEX IF NOT EXISTS idx_tickets_priority ON tickets(priority);
	CREATE INDEX IF NOT EXISTS idx_tickets_assignee ON tickets(assignee);
	`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to run database migration: %w", err)
	}

	return nil
}

