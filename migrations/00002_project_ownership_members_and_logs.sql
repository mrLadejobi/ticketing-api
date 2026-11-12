-- +goose Up
ALTER TABLE projects
	ADD COLUMN IF NOT EXISTS creator_id INT REFERENCES users(id) ON DELETE SET NULL,
	ADD COLUMN IF NOT EXISTS start_date TIMESTAMP WITH TIME ZONE DEFAULT NULL,
	ADD COLUMN IF NOT EXISTS due_date TIMESTAMP WITH TIME ZONE DEFAULT NULL,
	ADD COLUMN IF NOT EXISTS progress_percent SMALLINT DEFAULT NULL
		CONSTRAINT chk_project_progress CHECK (progress_percent IS NULL OR (progress_percent BETWEEN 0 AND 100));

CREATE INDEX IF NOT EXISTS idx_projects_creator ON projects(creator_id);
CREATE INDEX IF NOT EXISTS idx_projects_due_date ON projects(due_date);

CREATE TABLE IF NOT EXISTS project_members (
	id SERIAL PRIMARY KEY,
	project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	role VARCHAR(20) NOT NULL DEFAULT 'MEMBER',
	added_by INT REFERENCES users(id) ON DELETE SET NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT uq_project_member UNIQUE (project_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members(project_id);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);

CREATE TABLE IF NOT EXISTS project_logs (
	id SERIAL PRIMARY KEY,
	project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	actor_id INT REFERENCES users(id) ON DELETE SET NULL,
	ticket_id INT REFERENCES tickets(id) ON DELETE SET NULL,
	action VARCHAR(60) NOT NULL,
	summary TEXT NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_project_logs_project_created ON project_logs(project_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS project_logs;
DROP TABLE IF EXISTS project_members;
ALTER TABLE projects
	DROP COLUMN IF EXISTS progress_percent,
	DROP COLUMN IF EXISTS due_date,
	DROP COLUMN IF EXISTS start_date,
	DROP COLUMN IF EXISTS creator_id;

