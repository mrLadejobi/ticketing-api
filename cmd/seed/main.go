package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"ticketing-api/internal/config"
	"ticketing-api/internal/database"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type UserSeed struct {
	Email    string
	Password string
	FullName string
	Role     string
}

func main() {
	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database for seeding: %v\nEnsure PostgreSQL is running at %s", err, cfg.DSN())
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	ctx := context.Background()
	log.Println("🌱 Seeding enterprise database with realistic engineering data...")

	// 1. Seed Users
	users := []UserSeed{
		{"admin@linear-enterprise.internal", "AdminPass123!", "System Administrator", "ADMIN"},
		{"sarah.lead@company.com", "Engineer123!", "Sarah Connor", "MEMBER"},
		{"alex.dev@company.com", "Engineer123!", "Alex Mercer", "MEMBER"},
		{"priya.backend@company.com", "Engineer123!", "Priya Sharma", "MEMBER"},
		{"marcus.qa@company.com", "Engineer123!", "Marcus Brody", "MEMBER"},
		{"elena.pm@company.com", "Product123!", "Elena Rostova", "MEMBER"},
	}

	userMap := make(map[string]int) // email -> id

	for _, u := range users {
		hash, _ := bcrypt.GenerateFromPassword([]byte(u.Password), 10)
		var id int
		query := `
			INSERT INTO users (email, password_hash, full_name, role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (email) DO UPDATE SET full_name = EXCLUDED.full_name
			RETURNING id
		`
		if err := db.QueryRowContext(ctx, query, u.Email, string(hash), u.FullName, u.Role).Scan(&id); err != nil {
			log.Fatalf("Failed to seed user %s: %v", u.Email, err)
		}
		userMap[u.Email] = id
	}

	// 2. Seed Personal Access Token (PAT) for Admin
	adminID := userMap["admin@linear-enterprise.internal"]
	patBytes := make([]byte, 24)
	_, _ = rand.Read(patBytes)
	rawPAT := "pat_" + hex.EncodeToString(patBytes)
	hasher := sha256.New()
	hasher.Write([]byte(rawPAT))
	patHash := hex.EncodeToString(hasher.Sum(nil))

	_, _ = db.ExecContext(ctx, `
		INSERT INTO api_keys (user_id, name, prefix, key_hash, created_at)
		VALUES ($1, 'Master Demo CLI Token', $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (key_hash) DO NOTHING
	`, adminID, rawPAT[:10], patHash)

	// 3. Seed Teams
	type TeamSeed struct {
		Name string
		Slug string
		Desc string
	}
	teams := []TeamSeed{
		{"Core Platform", "core-platform", "Platform foundation, distributed consensus, auth & tenant services"},
		{"Payments & Ledger", "payments-ledger", "PCI-compliant double-entry ledger, Stripe webhooks, settlement pipelines"},
	}
	teamMap := make(map[string]int) // slug -> id
	for _, t := range teams {
		var id int
		err := db.QueryRowContext(ctx, `
			INSERT INTO teams (name, slug, description, created_at, updated_at)
			VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, t.Name, t.Slug, t.Desc).Scan(&id)
		if err != nil {
			log.Fatalf("Failed to seed team %s: %v", t.Slug, err)
		}
		teamMap[t.Slug] = id
	}

	// 4. Seed Team Memberships
	memberships := []struct {
		TeamSlug  string
		UserEmail string
		Role      string
	}{
		{"core-platform", "sarah.lead@company.com", "LEAD"},
		{"core-platform", "alex.dev@company.com", "DEVELOPER"},
		{"core-platform", "priya.backend@company.com", "DEVELOPER"},
		{"core-platform", "elena.pm@company.com", "STAKEHOLDER"},
		{"payments-ledger", "alex.dev@company.com", "LEAD"},
		{"payments-ledger", "priya.backend@company.com", "DEVELOPER"},
		{"payments-ledger", "marcus.qa@company.com", "DEVELOPER"},
		{"payments-ledger", "elena.pm@company.com", "STAKEHOLDER"},
	}
	for _, m := range memberships {
		_, _ = db.ExecContext(ctx, `
			INSERT INTO team_members (team_id, user_id, role, created_at)
			VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
			ON CONFLICT (team_id, user_id) DO NOTHING
		`, teamMap[m.TeamSlug], userMap[m.UserEmail], m.Role)
	}

	// 5. Seed Projects
	type ProjectSeed struct {
		TeamSlug string
		Name     string
		Prefix   string
	}
	projects := []ProjectSeed{
		{"core-platform", "Platform Foundation & Core Services", "ENG"},
		{"payments-ledger", "Payments Gateway & Double-Entry Ledger", "PAY"},
		{"core-platform", "Cloud Infrastructure, Kubernetes & SRE", "INFRA"},
	}
	projMap := make(map[string]int) // prefix -> id
	for _, p := range projects {
		var id int
		err := db.QueryRowContext(ctx, `
			INSERT INTO projects (team_id, name, prefix, issue_counter, created_at, updated_at)
			VALUES ($1, $2, $3, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (prefix) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, teamMap[p.TeamSlug], p.Name, p.Prefix).Scan(&id)
		if err != nil {
			log.Fatalf("Failed to seed project %s: %v", p.Prefix, err)
		}
		projMap[p.Prefix] = id
	}

	// 6. Seed Project Labels
	type LabelSeed struct {
		Name  string
		Color string
	}
	stdLabels := []LabelSeed{
		{"bug", "#E5484D"},
		{"p0-critical", "#E54666"},
		{"tech-debt", "#8E4EC6"},
		{"security", "#30A46C"},
		{"feature", "#0091FF"},
	}
	labelMap := make(map[string]map[string]int) // projPrefix -> labelName -> id
	for prefix, pID := range projMap {
		labelMap[prefix] = make(map[string]int)
		for _, l := range stdLabels {
			var lid int
			err := db.QueryRowContext(ctx, `
				INSERT INTO project_labels (project_id, name, color_hex, created_at)
				VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
				ON CONFLICT (project_id, name) DO UPDATE SET color_hex = EXCLUDED.color_hex
				RETURNING id
			`, pID, l.Name, l.Color).Scan(&lid)
			if err == nil {
				labelMap[prefix][l.Name] = lid
			}
		}
	}

	// 7. Seed Agile Cycles (Sprints)
	now := time.Now().UTC()
	type CycleSeed struct {
		ProjectPrefix string
		Name          string
		Goal          string
		Start         time.Time
		End           time.Time
		Status        string
	}
	cycles := []CycleSeed{
		{"ENG", "ENG Sprint 31 (Retrospective)", "Distributed tracing, rate limiting & v2 session cookies", now.Add(-28 * 24 * time.Hour), now.Add(-14 * 24 * time.Hour), "COMPLETED"},
		{"ENG", "ENG Sprint 32 (Active)", "SSE real-time events hub, bulk mutations & audit trails", now.Add(-2 * 24 * time.Hour), now.Add(12 * 24 * time.Hour), "ACTIVE"},
		{"PAY", "PAY Sprint 14 (Settlements)", "Automated wire reconciliations and merchant ledger", now.Add(-20 * 24 * time.Hour), now.Add(-6 * 24 * time.Hour), "COMPLETED"},
		{"PAY", "PAY Sprint 15 (Gateway V2)", "Idempotent webhooks, Stripe/Adyen dual routing, chargeback SLA", now.Add(-1 * 24 * time.Hour), now.Add(13 * 24 * time.Hour), "ACTIVE"},
		{"INFRA", "INFRA Q4 Migration", "Zero-downtime PostgreSQL 16 cluster upgrade & read replicas", now.Add(-40 * 24 * time.Hour), now.Add(-10 * 24 * time.Hour), "COMPLETED"},
		{"INFRA", "INFRA Q1 Kubernetes 1.30", "EKS cluster upgrades, Cilium eBPF mesh, telemetry dashboards", now.Add(-5 * 24 * time.Hour), now.Add(15 * 24 * time.Hour), "ACTIVE"},
	}
	cycleMap := make(map[string]int) // "ENG:ENG Sprint 32 (Active)" -> id
	for _, c := range cycles {
		var cid int
		pID := projMap[c.ProjectPrefix]
		err := db.QueryRowContext(ctx, `
			INSERT INTO project_cycles (project_id, name, goal, start_date, end_date, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT DO NOTHING
			RETURNING id
		`, pID, c.Name, c.Goal, c.Start, c.End, c.Status).Scan(&cid)
		if err == nil {
			cycleMap[c.ProjectPrefix+":"+c.Name] = cid
		} else {
			// Query existing
			_ = db.QueryRowContext(ctx, `SELECT id FROM project_cycles WHERE project_id = $1 AND name = $2`, pID, c.Name).Scan(&cid)
			cycleMap[c.ProjectPrefix+":"+c.Name] = cid
		}
	}

	// 8. Seed 27 Software Engineering Tickets (All 9 statuses, subtasks, relations, and comments)
	type TicketSeed struct {
		ProjectPrefix string
		KeyNum        int
		Title         string
		Desc          string
		Status        string
		Priority      string
		Reporter      string
		Assignee      string
		CycleKey      string
		Labels        []string
		ParentKeyNum  *int
	}

	parent1 := 1  // ENG-1 is epic
	parent11 := 1 // PAY-11 is epic

	tickets := []TicketSeed{
		// ENG Project Tickets
		{"ENG", 1, "Epic: Distributed Authentication & Session Management", "Overhaul token refresh architecture, HttpOnly session rotation, and multi-tenant scoping.", "IN_PROGRESS", "URGENT", "elena.pm@company.com", "sarah.lead@company.com", "ENG:ENG Sprint 32 (Active)", []string{"feature", "security"}, nil},
		{"ENG", 2, "Implement AES-256 session token encryption at rest", "Encrypt refresh token fingerprints inside the secure key rotation vault.", "DONE", "HIGH", "sarah.lead@company.com", "alex.dev@company.com", "ENG:ENG Sprint 31 (Retrospective)", []string{"security"}, &parent1},
		{"ENG", 3, "Add Redis-backed sliding window rate limiter", "Protect /api/v1/auth/login against distributed credential stuffing attacks.", "IN_REVIEW", "HIGH", "alex.dev@company.com", "priya.backend@company.com", "ENG:ENG Sprint 32 (Active)", []string{"security", "feature"}, &parent1},
		{"ENG", 4, "Server-Sent Events (SSE) notification streaming hub", "Build thread-safe channel broker with 25s ping heartbeats for real-time live events.", "QA_TESTING", "HIGH", "elena.pm@company.com", "priya.backend@company.com", "ENG:ENG Sprint 32 (Active)", []string{"feature"}, &parent1},
		{"ENG", 5, "Fix optimistic concurrency locking race on bulk ticket patch", "Ensure If-Match / version check atomically prevents dirty reads under high load.", "DONE", "MEDIUM", "priya.backend@company.com", "alex.dev@company.com", "ENG:ENG Sprint 31 (Retrospective)", []string{"bug"}, nil},
		{"ENG", 6, "Add Markdown syntax parser for comment @mentions", "Extract email handles and notify mentioned collaborators asynchronously.", "DONE", "LOW", "sarah.lead@company.com", "alex.dev@company.com", "ENG:ENG Sprint 31 (Retrospective)", []string{"feature"}, nil},
		{"ENG", 7, "Investigate memory spike during 10k batch CSV export", "Profiling indicates excessive allocation in JSON serializer during pagination.", "TRIAGE", "MEDIUM", "marcus.qa@company.com", "alex.dev@company.com", "", []string{"bug", "tech-debt"}, nil},
		{"ENG", 8, "Design schema migration for custom workflow automations", "Support user-defined webhooks triggers upon status change.", "BACKLOG", "LOW", "elena.pm@company.com", "", "", []string{"feature"}, nil},
		{"ENG", 9, "Flaky integration test in multi-tenant project isolation", "Intermittent deadlock when parallel test runner touches same team member table.", "BLOCKED", "HIGH", "marcus.qa@company.com", "sarah.lead@company.com", "ENG:ENG Sprint 32 (Active)", []string{"bug"}, nil},
		{"ENG", 10, "Deprecate legacy basic auth endpoint in v1 gateway", "Remove unused basic authentication middleware across all staging ingress.", "CANCELED", "LOW", "sarah.lead@company.com", "alex.dev@company.com", "", []string{"tech-debt"}, nil},

		// PAY Project Tickets
		{"PAY", 11, "Epic: PCI-DSS Compliant Double-Entry Ledger Engine", "Zero-loss financial accounting ledger with append-only ledger entries and atomic multi-account transfers.", "IN_PROGRESS", "URGENT", "elena.pm@company.com", "alex.dev@company.com", "PAY:PAY Sprint 15 (Gateway V2)", []string{"feature", "p0-critical"}, nil},
		{"PAY", 12, "Idempotent Stripe webhook receiver with replay deduplication", "Deduplicate webhook event IDs using distributed lock and SHA-256 idempotency cache.", "DONE", "URGENT", "alex.dev@company.com", "priya.backend@company.com", "PAY:PAY Sprint 14 (Settlements)", []string{"p0-critical", "security"}, &parent11},
		{"PAY", 13, "Automated merchant payout fee calculation engine", "Calculate basis-points interchange plus margin fees on multi-currency payouts.", "TODO", "HIGH", "elena.pm@company.com", "priya.backend@company.com", "PAY:PAY Sprint 15 (Gateway V2)", []string{"feature"}, &parent11},
		{"PAY", 14, "Adyen fallback routing for European credit card declines", "Smart routing when primary gateway returns transient 504 gateway timeout.", "IN_REVIEW", "HIGH", "priya.backend@company.com", "alex.dev@company.com", "PAY:PAY Sprint 15 (Gateway V2)", []string{"feature"}, &parent11},
		{"PAY", 15, "Chargeback evidence PDF generator and automated portal submission", "Generate encrypted dispute package for VISA/Mastercard portal with S3 presigned URLs.", "BACKLOG", "MEDIUM", "elena.pm@company.com", "", "", []string{"feature"}, nil},
		{"PAY", 16, "Fix negative balance edge-case during simultaneous split tenders", "Enforce strict row-level lock on account balances during transfer transactions.", "DONE", "URGENT", "marcus.qa@company.com", "priya.backend@company.com", "PAY:PAY Sprint 14 (Settlements)", []string{"bug", "p0-critical"}, nil},
		{"PAY", 17, "Resolve ledger reconciliation mismatch on JPY rounding", "Zero-decimal currencies must truncate fractional fee calculations.", "TRIAGE", "HIGH", "marcus.qa@company.com", "alex.dev@company.com", "", []string{"bug"}, nil},
		{"PAY", 18, "Vendor payout API authentication via Personal Access Tokens", "Enable external ERP automated settlement calls with Bearer pat_ keys.", "DONE", "MEDIUM", "alex.dev@company.com", "sarah.lead@company.com", "PAY:PAY Sprint 14 (Settlements)", []string{"feature"}, nil},

		// INFRA Project Tickets
		{"INFRA", 19, "Zero-downtime PostgreSQL 16 cluster failover testing", "Execute automated chaos drills with Patroni and etcd split-brain recovery.", "DONE", "URGENT", "sarah.lead@company.com", "priya.backend@company.com", "INFRA:INFRA Q4 Migration", []string{"p0-critical", "tech-debt"}, nil},
		{"INFRA", 20, "Migrate Kubernetes ingress controllers from NGINX to Envoy Gateway", "Leverage Gateway API specs with dynamic TLS SNI certificate reloads.", "IN_PROGRESS", "HIGH", "priya.backend@company.com", "sarah.lead@company.com", "INFRA:INFRA Q1 Kubernetes 1.30", []string{"feature", "tech-debt"}, nil},
		{"INFRA", 21, "Cilium eBPF network security observability and wireguard encryption", "Node-to-node pod transit encryption with sub-millisecond overhead.", "IN_REVIEW", "HIGH", "priya.backend@company.com", "sarah.lead@company.com", "INFRA:INFRA Q1 Kubernetes 1.30", []string{"security"}, nil},
		{"INFRA", 22, "Prometheus alert rules for DB connection pool exhaustion", "Trigger PagerDuty when pgxpool idle connections drop below 15% for > 60s.", "DONE", "MEDIUM", "sarah.lead@company.com", "priya.backend@company.com", "INFRA:INFRA Q4 Migration", []string{"tech-debt"}, nil},
		{"INFRA", 23, "Upgrade GitHub Actions self-hosted runners to ARM64", "Switch build infrastructure to Apple Silicon runners to cut compile times in half.", "TODO", "LOW", "alex.dev@company.com", "priya.backend@company.com", "INFRA:INFRA Q1 Kubernetes 1.30", []string{"feature"}, nil},
		{"INFRA", 24, "S3 backup lifecycle policy for cold glacier storage", "Automate database WAL logs archival to Glacier Instant Retrieval after 30 days.", "DONE", "LOW", "sarah.lead@company.com", "priya.backend@company.com", "INFRA:INFRA Q4 Migration", []string{"tech-debt"}, nil},
		{"INFRA", 25, "Container vulnerability CVE scanning in CI pipeline", "Block merges if Trivy detects HIGH or CRITICAL base image vulnerabilities.", "TODO", "HIGH", "marcus.qa@company.com", "sarah.lead@company.com", "INFRA:INFRA Q1 Kubernetes 1.30", []string{"security"}, nil},
		{"INFRA", 26, "Investigate high DNS resolution latency inside pod namespaces", "CoreDNS response latency spikes during simultaneous service scaling.", "BLOCKED", "MEDIUM", "marcus.qa@company.com", "priya.backend@company.com", "INFRA:INFRA Q1 Kubernetes 1.30", []string{"bug"}, nil},
		{"INFRA", 27, "Decommission deprecated Redis 6.2 standalone caching instance", "All workloads successfully migrated to AWS ElastiCache cluster.", "CANCELED", "LOW", "sarah.lead@company.com", "alex.dev@company.com", "", []string{"tech-debt"}, nil},
	}

	ticketIDMap := make(map[string]int) // "ENG-1" -> id

	for _, t := range tickets {
		pID := projMap[t.ProjectPrefix]
		key := fmt.Sprintf("%s-%d", t.ProjectPrefix, t.KeyNum)
		repID := userMap[t.Reporter]

		var assID *int
		if t.Assignee != "" {
			aid := userMap[t.Assignee]
			assID = &aid
		}

		var cycID *int
		if t.CycleKey != "" {
			if cid, ok := cycleMap[t.CycleKey]; ok {
				cycID = &cid
			}
		}

		var parentID *int
		if t.ParentKeyNum != nil {
			parentKey := fmt.Sprintf("%s-%d", t.ProjectPrefix, *t.ParentKeyNum)
			if pid, ok := ticketIDMap[parentKey]; ok {
				parentID = &pid
			}
		}

		var tid int
		err := db.QueryRowContext(ctx, `
			INSERT INTO tickets (
				project_id, parent_id, cycle_id, key, title, description, status, priority,
				reporter_id, assignee_id, version, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT DO NOTHING
			RETURNING id
		`, pID, parentID, cycID, key, t.Title, t.Desc, t.Status, t.Priority, repID, assID).Scan(&tid)
		if err != nil {
			_ = db.QueryRowContext(ctx, `SELECT id FROM tickets WHERE key = $1`, key).Scan(&tid)
		}
		ticketIDMap[key] = tid

		// Update atomic project issue counter
		_, _ = db.ExecContext(ctx, `UPDATE projects SET issue_counter = GREATEST(issue_counter, $1) WHERE id = $2`, t.KeyNum, pID)

		// Attach labels
		for _, lblName := range t.Labels {
			if lid, ok := labelMap[t.ProjectPrefix][lblName]; ok {
				_, _ = db.ExecContext(ctx, `
					INSERT INTO ticket_labels (ticket_id, label_id, created_at)
					VALUES ($1, $2, CURRENT_TIMESTAMP)
					ON CONFLICT DO NOTHING
				`, tid, lid)
			}
		}

		// Add Activity Ledger Row
		_, _ = db.ExecContext(ctx, `
			INSERT INTO ticket_activities (ticket_id, actor_id, field, old_value, new_value, created_at)
			VALUES ($1, $2, 'status', 'TODO', $3, CURRENT_TIMESTAMP)
		`, tid, repID, t.Status)
	}

	// 9. Seed Issue Relations (BLOCKS / IS_BLOCKED_BY)
	relations := []struct {
		SourceKey string
		TargetKey string
		RelType   string
	}{
		{"ENG-9", "ENG-4", "BLOCKS"},
		{"PAY-16", "PAY-13", "BLOCKS"},
		{"INFRA-26", "INFRA-21", "BLOCKS"},
		{"ENG-2", "ENG-3", "RELATES_TO"},
		{"PAY-12", "PAY-18", "RELATES_TO"},
	}
	for _, r := range relations {
		sID := ticketIDMap[r.SourceKey]
		tID := ticketIDMap[r.TargetKey]
		if sID > 0 && tID > 0 {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO issue_relations (source_ticket_id, target_ticket_id, relation_type, created_at)
				VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
				ON CONFLICT DO NOTHING
			`, sID, tID, r.RelType)
		}
	}

	// 10. Seed Realistic Collaboration Comments with @Mentions
	comments := []struct {
		TicketKey string
		Author    string
		Body      string
	}{
		{"ENG-1", "sarah.lead@company.com", "Kicking off the architecture review. @alex.dev@company.com please ensure token rotation uses cryptographic random entropy and short lifetimes."},
		{"ENG-1", "alex.dev@company.com", "Agreed @sarah.lead@company.com. We have benchmarked SHA-256 for API keys and 15-minute access tokens with HttpOnly refresh cookies."},
		{"ENG-3", "priya.backend@company.com", "PR is ready for review at https://github.com/company/platform/pull/104. Verified rate limits prevent brute force spikes."},
		{"PAY-12", "alex.dev@company.com", "Stripe webhook deduplication is live in staging. Replay attack simulations successfully filtered duplicate deliveries."},
		{"INFRA-20", "sarah.lead@company.com", "Envoy Gateway configuration validated against our canary cluster. Ingress latency dropped by 22ms."},
	}
	for _, c := range comments {
		tID := ticketIDMap[c.TicketKey]
		aID := userMap[c.Author]
		if tID > 0 && aID > 0 {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO ticket_comments (ticket_id, author_id, body, mentions, created_at, updated_at)
				VALUES ($1, $2, $3, '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			`, tID, aID, c.Body)
		}
	}

	// 11. Seed In-App Notifications
	sarahID := userMap["sarah.lead@company.com"]
	alexID := userMap["alex.dev@company.com"]
	t1ID := ticketIDMap["ENG-1"]
	_, _ = db.ExecContext(ctx, `
		INSERT INTO notifications (user_id, actor_id, ticket_id, type, title, body, is_read, created_at)
		VALUES
			($1, $2, $3, 'MENTION', 'Sarah mentioned you in ENG-1', 'Kicking off the architecture review. @alex.dev@company.com please ensure token rotation...', FALSE, CURRENT_TIMESTAMP),
			($1, $2, $3, 'ASSIGNED', 'Assigned to ENG-1', 'You were assigned as lead developer on Epic ENG-1.', TRUE, CURRENT_TIMESTAMP - INTERVAL '1 hour')
	`, alexID, sarahID, t1ID)

	// Summary Banner Output
	divider := strings.Repeat("=", 78)
	fmt.Printf("\n%s\n", divider)
	fmt.Println("🚀 ENTERPRISE SEED COMPLETED SUCCESSFULLY")
	fmt.Printf("%s\n", divider)
	fmt.Printf("• Database:         %s:%s/%s\n", cfg.DBHost, cfg.DBPort, cfg.DBName)
	fmt.Printf("• Total Users:      %d (1 Admin, 4 Engineers, 1 Product Manager)\n", len(users))
	fmt.Printf("• Teams:            2 (Core Platform, Payments & Ledger)\n")
	fmt.Printf("• Projects:         3 (ENG, PAY, INFRA)\n")
	fmt.Printf("• Agile Sprints:    6 (3 COMPLETED, 3 ACTIVE)\n")
	fmt.Printf("• Seeded Tickets:   %d (Covering all 9 statuses, subtasks, BLOCKS relations)\n", len(tickets))
	fmt.Printf("• Audit History:    Immutable activity rows & notifications recorded\n")
	fmt.Printf("%s\n", divider)
	fmt.Println("🔑 DEMO LOGIN CREDENTIALS:")
	fmt.Println("------------------------------------------------------------------------------")
	fmt.Printf("%-35s %-16s %-10s\n", "Email", "Password", "Role")
	fmt.Println("------------------------------------------------------------------------------")
	for _, u := range users {
		fmt.Printf("%-35s %-16s %-10s\n", u.Email, u.Password, u.Role)
	}
	fmt.Println("------------------------------------------------------------------------------")
	fmt.Println("🔑 PERSONAL ACCESS TOKEN (PAT) FOR AUTOMATION / CLI / CI-CD:")
	fmt.Printf("  Authorization: Bearer %s\n", rawPAT)
	fmt.Println("------------------------------------------------------------------------------")
	fmt.Println("Interactive API Reference: http://localhost:8080/docs (Scalar UI)")
	fmt.Printf("%s\n\n", divider)
}
