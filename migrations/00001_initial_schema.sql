-- +goose Up
-- 1. Users Table
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	email VARCHAR(255) UNIQUE NOT NULL,
	password_hash VARCHAR(255) NOT NULL,
	full_name VARCHAR(100) NOT NULL,
	role VARCHAR(20) NOT NULL DEFAULT 'MEMBER',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- 2. Teams Table
CREATE TABLE IF NOT EXISTS teams (
	id SERIAL PRIMARY KEY,
	name VARCHAR(100) NOT NULL,
	slug VARCHAR(100) UNIQUE NOT NULL,
	description TEXT DEFAULT '',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_teams_slug ON teams(slug);

-- 3. Team Members Table (RBAC per team)
CREATE TABLE IF NOT EXISTS team_members (
	id SERIAL PRIMARY KEY,
	team_id INT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	role VARCHAR(20) NOT NULL DEFAULT 'DEVELOPER',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT uq_team_user UNIQUE (team_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_team_members_team ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user ON team_members(user_id);

-- 4. Projects Table
CREATE TABLE IF NOT EXISTS projects (
	id SERIAL PRIMARY KEY,
	team_id INT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	name VARCHAR(100) NOT NULL,
	prefix VARCHAR(10) UNIQUE NOT NULL,
	description TEXT DEFAULT '',
	issue_counter INT NOT NULL DEFAULT 0,
	status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_projects_team ON projects(team_id);
CREATE INDEX IF NOT EXISTS idx_projects_prefix ON projects(prefix);

-- 5. Project Cycles / Sprints Table
CREATE TABLE IF NOT EXISTS project_cycles (
	id SERIAL PRIMARY KEY,
	project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name VARCHAR(100) NOT NULL,
	goal TEXT DEFAULT '',
	start_date TIMESTAMP WITH TIME ZONE NOT NULL,
	end_date TIMESTAMP WITH TIME ZONE NOT NULL,
	status VARCHAR(20) NOT NULL DEFAULT 'PLANNED',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cycles_project ON project_cycles(project_id);
CREATE INDEX IF NOT EXISTS idx_cycles_status ON project_cycles(status);

-- 6. Tickets Table
CREATE TABLE IF NOT EXISTS tickets (
	id SERIAL PRIMARY KEY,
	project_id INT REFERENCES projects(id) ON DELETE CASCADE,
	parent_id INT REFERENCES tickets(id) ON DELETE SET NULL,
	cycle_id INT REFERENCES project_cycles(id) ON DELETE SET NULL,
	key VARCHAR(20) UNIQUE NOT NULL,
	title VARCHAR(200) NOT NULL,
	description TEXT NOT NULL,
	status VARCHAR(30) NOT NULL DEFAULT 'TODO',
	priority VARCHAR(20) NOT NULL DEFAULT 'MEDIUM',
	reporter_id INT NOT NULL REFERENCES users(id),
	assignee_id INT REFERENCES users(id),
	version INT NOT NULL DEFAULT 1,
	deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tickets_project ON tickets(project_id);
CREATE INDEX IF NOT EXISTS idx_tickets_parent ON tickets(parent_id);
CREATE INDEX IF NOT EXISTS idx_tickets_cycle ON tickets(cycle_id);
CREATE INDEX IF NOT EXISTS idx_tickets_key ON tickets(key);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_priority ON tickets(priority);
CREATE INDEX IF NOT EXISTS idx_tickets_assignee ON tickets(assignee_id);
CREATE INDEX IF NOT EXISTS idx_tickets_reporter ON tickets(reporter_id);
CREATE INDEX IF NOT EXISTS idx_tickets_deleted_at ON tickets(deleted_at);

-- 7. Ticket Comments Table
CREATE TABLE IF NOT EXISTS ticket_comments (
	id SERIAL PRIMARY KEY,
	ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	author_id INT NOT NULL REFERENCES users(id),
	body TEXT NOT NULL,
	mentions TEXT[] DEFAULT '{}',
	deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_ticket ON ticket_comments(ticket_id);
CREATE INDEX IF NOT EXISTS idx_comments_author ON ticket_comments(author_id);

-- 8. Ticket Watchers Table
CREATE TABLE IF NOT EXISTS ticket_watchers (
	id SERIAL PRIMARY KEY,
	ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT uq_ticket_watcher UNIQUE (ticket_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_watchers_ticket ON ticket_watchers(ticket_id);
CREATE INDEX IF NOT EXISTS idx_watchers_user ON ticket_watchers(user_id);

-- 9. Ticket Attachments Table
CREATE TABLE IF NOT EXISTS ticket_attachments (
	id SERIAL PRIMARY KEY,
	ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	uploader_id INT NOT NULL REFERENCES users(id),
	file_name VARCHAR(255) NOT NULL,
	file_size BIGINT NOT NULL,
	content_type VARCHAR(100) NOT NULL,
	file_url VARCHAR(500) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_attachments_ticket ON ticket_attachments(ticket_id);

-- 10. Ticket Relations Table
CREATE TABLE IF NOT EXISTS ticket_relations (
	id SERIAL PRIMARY KEY,
	source_ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	target_ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	relation_type VARCHAR(30) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT uq_ticket_relation UNIQUE (source_ticket_id, target_ticket_id, relation_type),
	CONSTRAINT chk_no_self_relation CHECK (source_ticket_id <> target_ticket_id)
);
CREATE INDEX IF NOT EXISTS idx_relations_source ON ticket_relations(source_ticket_id);
CREATE INDEX IF NOT EXISTS idx_relations_target ON ticket_relations(target_ticket_id);

-- 11. Project Labels Table
CREATE TABLE IF NOT EXISTS project_labels (
	id SERIAL PRIMARY KEY,
	project_id INT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name VARCHAR(50) NOT NULL,
	color_hex VARCHAR(10) NOT NULL DEFAULT '#6B7280',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT uq_project_label_name UNIQUE (project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_labels_project ON project_labels(project_id);

-- 12. Ticket Labels Join Table
CREATE TABLE IF NOT EXISTS ticket_labels (
	id SERIAL PRIMARY KEY,
	ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	label_id INT NOT NULL REFERENCES project_labels(id) ON DELETE CASCADE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	CONSTRAINT uq_ticket_label UNIQUE (ticket_id, label_id)
);
CREATE INDEX IF NOT EXISTS idx_ticket_labels_ticket ON ticket_labels(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_labels_label ON ticket_labels(label_id);

-- 13. Ticket Activities Table
CREATE TABLE IF NOT EXISTS ticket_activities (
	id SERIAL PRIMARY KEY,
	ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	actor_id INT REFERENCES users(id) ON DELETE SET NULL,
	field VARCHAR(50) NOT NULL,
	old_value TEXT DEFAULT '',
	new_value TEXT DEFAULT '',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activities_ticket ON ticket_activities(ticket_id);
CREATE INDEX IF NOT EXISTS idx_activities_actor ON ticket_activities(actor_id);
CREATE INDEX IF NOT EXISTS idx_activities_created ON ticket_activities(created_at);

-- 14. System-Wide Audit Logs Table
CREATE TABLE IF NOT EXISTS audit_logs (
	id SERIAL PRIMARY KEY,
	actor_id INT REFERENCES users(id) ON DELETE SET NULL,
	action VARCHAR(100) NOT NULL,
	resource_type VARCHAR(50) NOT NULL,
	resource_id VARCHAR(100) NOT NULL,
	payload TEXT DEFAULT '{}',
	ip_address VARCHAR(50) DEFAULT '',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);

-- 15. API Keys Table
CREATE TABLE IF NOT EXISTS api_keys (
	id SERIAL PRIMARY KEY,
	user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name VARCHAR(100) NOT NULL,
	prefix VARCHAR(15) NOT NULL,
	key_hash VARCHAR(128) UNIQUE NOT NULL,
	last_used_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
	expires_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);

-- 16. Webhook Deliveries Table
CREATE TABLE IF NOT EXISTS webhook_deliveries (
	delivery_id VARCHAR(255) PRIMARY KEY,
	event_type VARCHAR(100) DEFAULT '',
	processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_webhook_processed ON webhook_deliveries(processed_at);

-- 17. In-App Notifications Table
CREATE TABLE IF NOT EXISTS notifications (
	id SERIAL PRIMARY KEY,
	user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	actor_id INT REFERENCES users(id) ON DELETE SET NULL,
	ticket_id INT REFERENCES tickets(id) ON DELETE CASCADE,
	type VARCHAR(50) NOT NULL,
	title VARCHAR(255) NOT NULL,
	body TEXT DEFAULT '',
	is_read BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at);

-- +goose Down
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS ticket_activities;
DROP TABLE IF EXISTS ticket_labels;
DROP TABLE IF EXISTS project_labels;
DROP TABLE IF EXISTS ticket_relations;
DROP TABLE IF EXISTS ticket_attachments;
DROP TABLE IF EXISTS ticket_watchers;
DROP TABLE IF EXISTS ticket_comments;
DROP TABLE IF EXISTS tickets;
DROP TABLE IF EXISTS project_cycles;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;

