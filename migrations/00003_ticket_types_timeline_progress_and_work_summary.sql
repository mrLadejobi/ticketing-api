-- +goose Up
ALTER TABLE tickets
	ADD COLUMN IF NOT EXISTS ticket_type VARCHAR(20) NOT NULL DEFAULT 'TASK',
	ADD COLUMN IF NOT EXISTS due_date TIMESTAMP WITH TIME ZONE DEFAULT NULL,
	ADD COLUMN IF NOT EXISTS progress_percent SMALLINT DEFAULT NULL
		CONSTRAINT chk_ticket_progress CHECK (progress_percent IS NULL OR (progress_percent BETWEEN 0 AND 100)),
	ADD COLUMN IF NOT EXISTS work_summary TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_tickets_type ON tickets(ticket_type);
CREATE INDEX IF NOT EXISTS idx_tickets_due_date ON tickets(due_date);

-- +goose Down
DROP INDEX IF EXISTS idx_tickets_due_date;
DROP INDEX IF EXISTS idx_tickets_type;
ALTER TABLE tickets
	DROP COLUMN IF EXISTS work_summary,
	DROP COLUMN IF EXISTS progress_percent,
	DROP COLUMN IF EXISTS due_date,
	DROP COLUMN IF EXISTS ticket_type;

