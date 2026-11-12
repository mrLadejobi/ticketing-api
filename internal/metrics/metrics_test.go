package metrics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/metrics"
	"ticketing-api/internal/project"
	"ticketing-api/internal/ticket"
)

func TestUserAndProjectMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userRepo := auth.NewMemoryRepository()
	u, _ := userRepo.CreateUser(context.Background(), &auth.User{
		Email:    "alex@company.com",
		FullName: "Alex Mercer",
		Role:     auth.RoleMember,
	})

	projRepo := project.NewMemoryRepository(userRepo)
	projSvc := project.NewService(projRepo)
	proj, _ := projSvc.CreateProject(context.Background(), u.ID, project.CreateProjectRequest{
		TeamID: 1,
		Name:   "Core Platform",
		Prefix: "ENG",
	})

	ticketRepo := ticket.NewMemoryRepository(userRepo)
	ticketSvc := ticket.NewService(ticketRepo, projSvc)

	pastDue := time.Now().UTC().Add(-48 * time.Hour)
	prog100 := 100
	prog40 := 40

	// 1. Completed task with detailed work_summary
	_, _ = ticketSvc.Create(context.Background(), u.ID, ticket.CreateTicketRequest{
		ProjectID:       proj.ID,
		TicketType:      ticket.TypeFeature,
		Title:           "Build JWT Auth",
		Description:     "Implement access + refresh cookie rotation",
		WorkSummary:     "Implemented HttpOnly rotation, added 15m TTL, verified with unit tests.",
		Status:          ticket.StatusCompleted,
		Priority:        ticket.PriorityHigh,
		ProgressPercent: &prog100,
		AssigneeID:      &u.ID,
	})

	// 2. Overdue task needing immediate attention
	_, _ = ticketSvc.Create(context.Background(), u.ID, ticket.CreateTicketRequest{
		ProjectID:       proj.ID,
		TicketType:      ticket.TypeBug,
		Title:           "Fix connection leak",
		Description:     "DB pool exhausts under load",
		WorkSummary:     "Profiled goroutines, isolating idle connection timeout.",
		Status:          ticket.StatusInProgress,
		Priority:        ticket.PriorityUrgent,
		ProgressPercent: &prog40,
		DueDate:         &pastDue,
		AssigneeID:      &u.ID,
	})

	// 3. On-hold task
	_, _ = ticketSvc.Create(context.Background(), u.ID, ticket.CreateTicketRequest{
		ProjectID:   proj.ID,
		TicketType:  ticket.TypeTask,
		Title:       "Migrate legacy S3 bucket",
		Description: "Waiting on IAM approval",
		Status:      ticket.StatusOnHold,
		Priority:    ticket.PriorityLow,
		AssigneeID:  &u.ID,
	})

	router := gin.New()
	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", u.ID)
		c.Next()
	}
	h := metrics.NewHandler(ticketRepo, userRepo, projSvc)
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1, dummyAuth)

	// Test GET /api/v1/users/me/metrics
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var userRes struct {
		Data metrics.UserMetricsResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &userRes)
	if userRes.Data.TotalAssigned != 3 || userRes.Data.TasksCompleted != 1 || userRes.Data.TasksInProgress != 1 || userRes.Data.TasksOnHold != 1 {
		t.Fatalf("unexpected user metrics counts: %+v", userRes.Data)
	}
	if len(userRes.Data.TasksNeedingAttention) != 1 || userRes.Data.TasksNeedingAttention[0].Key != "ENG-2" {
		t.Fatalf("expected ENG-2 needing immediate attention, got %+v", userRes.Data.TasksNeedingAttention)
	}

	// Test GET /api/v1/projects/1/metrics
	pReq, _ := http.NewRequest(http.MethodGet, "/api/v1/projects/1/metrics", nil)
	pW := httptest.NewRecorder()
	router.ServeHTTP(pW, pReq)
	if pW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for project metrics, got %d", pW.Code)
	}

	var projRes struct {
		Data metrics.ProjectMetricsResponse `json:"data"`
	}
	_ = json.Unmarshal(pW.Body.Bytes(), &projRes)
	if projRes.Data.TotalTasks != 3 || projRes.Data.CompletedTasks != 1 || projRes.Data.OverdueTasks != 1 {
		t.Fatalf("unexpected project metrics: %+v", projRes.Data)
	}
	if len(projRes.Data.MemberStats) != 1 || projRes.Data.MemberStats[0].CompletedCount != 1 {
		t.Fatalf("unexpected member stats: %+v", projRes.Data.MemberStats)
	}
}

