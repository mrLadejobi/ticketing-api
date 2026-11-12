package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"ticketing-api/internal/activity"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/config"
	"ticketing-api/internal/database"
	"ticketing-api/internal/project"
	"ticketing-api/internal/team"
	"ticketing-api/internal/ticket"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	cfg := config.Load()
	if testDBName := os.Getenv("TEST_DB_NAME"); testDBName != "" {
		cfg.DBName = testDBName
	} else {
		cfg.DBName = "test_db"
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("skipping PostgreSQL integration test (test_db unreachable at %s:%s/%s): %v", cfg.DBHost, cfg.DBPort, cfg.DBName, err)
	}

	if err := database.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("goose migration failed on %s: %v", cfg.DBName, err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestPostgres_GooseMigrations_And_EndToEndPersistence(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// 1. Verify goose_db_version is at migration 4
	var maxVersion int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied = true`).Scan(&maxVersion); err != nil {
		t.Fatalf("failed to query goose_db_version: %v", err)
	}
	if maxVersion < 4 {
		t.Fatalf("expected goose migration version >= 4, got %d", maxVersion)
	}

	// 2. Create isolated test user, team, project, and ticket
	suffix := time.Now().UnixNano()
	authRepo := auth.NewPostgresRepository(db)
	user, err := authRepo.CreateUser(ctx, &auth.User{
		Email:        fmt.Sprintf("pgtest-%d@company.com", suffix),
		PasswordHash: "hashed",
		FullName:     "Postgres Test User",
		Role:         auth.RoleMember,
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	teamRepo := team.NewPostgresRepository(db)
	tm, err := teamRepo.CreateTeam(ctx, &team.Team{
		Name:        "Test Team",
		Slug:        fmt.Sprintf("test-team-%d", suffix),
		Description: "Integration test team",
	})
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM teams WHERE id = $1`, tm.ID)
	})

	projRepo := project.NewPostgresRepository(db)
	projSvc := project.NewService(projRepo)
	prefix := fmt.Sprintf("T%d", suffix%9000+1000)
	proj, err := projSvc.CreateProject(ctx, user.ID, project.CreateProjectRequest{
		TeamID: tm.ID,
		Name:   "Test DB Project",
		Prefix: prefix,
		Status: project.StatusStarted,
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// 3. Create FEATURE ticket with progress_percent, due_date, and work_summary
	ticketRepo := ticket.NewPostgresRepository(db)
	actRepo := activity.NewPostgresRepository(db)
	actSvc := activity.NewService(actRepo)

	ticketSvc := ticket.NewService(ticketRepo, projSvc)
	ticketSvc.SetTxManager(database.NewTxManager(db))
	ticketSvc.SetActivityRecorder(&testActivityAdapter{svc: actSvc})

	prog := 50
	due := time.Now().UTC().Add(48 * time.Hour)
	created, err := ticketSvc.Create(ctx, user.ID, ticket.CreateTicketRequest{
		ProjectID:       proj.ID,
		TicketType:      ticket.TypeFeature,
		Title:           "PostgreSQL Persistence Feature",
		Description:     "Verify real Postgres CRUD + goose schema",
		WorkSummary:     "Initial implementation verified against test_db.",
		Status:          ticket.StatusInProgress,
		Priority:        ticket.PriorityHigh,
		ProgressPercent: &prog,
		DueDate:         &due,
		AssigneeID:      &user.ID,
	})
	if err != nil {
		t.Fatalf("Create ticket failed: %v", err)
	}

	// 4. Update status inside TxManager and verify activity row was committed atomically
	done := ticket.StatusCompleted
	ctxWithUser := context.WithValue(ctx, "user_id", user.ID)
	updated, err := ticketSvc.Update(ctxWithUser, created.ID, ticket.UpdateTicketRequest{
		Status: &done,
	})
	if err != nil {
		t.Fatalf("Update ticket failed: %v", err)
	}
	if updated.Status != ticket.StatusCompleted {
		t.Fatalf("expected status COMPLETED, got %s", updated.Status)
	}

	activities, total, err := actSvc.ListTicketActivities(ctx, created.ID, 10, 0)
	if err != nil || total == 0 || activities[0].NewValue != string(ticket.StatusCompleted) {
		t.Fatalf("expected persisted ticket_activity row in test_db, got total=%d err=%v", total, err)
	}
}

type testActivityAdapter struct {
	svc *activity.Service
}

func (a *testActivityAdapter) RecordTicketActivity(ctx context.Context, ticketID int, actorID *int, field, oldValue, newValue string) error {
	_, err := a.svc.RecordTicketActivity(ctx, ticketID, actorID, field, oldValue, newValue)
	return err
}

