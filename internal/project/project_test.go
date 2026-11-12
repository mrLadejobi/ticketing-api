package project_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/project"
	"ticketing-api/internal/ticket"
)

type mockTicketLister struct {
	tickets []ticket.Ticket
}

func (m *mockTicketLister) ListByProject(ctx context.Context, projectID int) ([]ticket.Ticket, error) {
	return m.tickets, nil
}

func setupProjectRouter(userID int, role auth.Role, service *project.Service, lister project.TicketLister) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := project.NewHandler(service, lister)
	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", string(role))
		c.Next()
	}
	v1 := r.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)
	return r
}

func TestProject_Members_CreatorRemoval_Logs_And_Timeline(t *testing.T) {
	userRepo := auth.NewMemoryRepository()
	_, _ = userRepo.CreateUser(context.Background(), &auth.User{Email: "creator@company.com", FullName: "Creator", Role: auth.RoleMember})
	_, _ = userRepo.CreateUser(context.Background(), &auth.User{Email: "dev@company.com", FullName: "Developer", Role: auth.RoleMember})
	_, _ = userRepo.CreateUser(context.Background(), &auth.User{Email: "other@company.com", FullName: "Other", Role: auth.RoleMember})

	repo := project.NewMemoryRepository(userRepo)
	service := project.NewService(repo)

	pastDue := time.Now().UTC().Add(-24 * time.Hour)
	futureDue := time.Now().UTC().Add(72 * time.Hour)
	prog := 40
	lister := &mockTicketLister{
		tickets: []ticket.Ticket{
			{ID: 1, ProjectID: 1, Key: "ENG-1", Title: "Overdue Bug", TicketType: ticket.TypeBug, Status: ticket.StatusInProgress, Priority: ticket.PriorityUrgent, DueDate: &pastDue, ProgressPercent: &prog},
			{ID: 2, ProjectID: 1, Key: "ENG-2", Title: "Upcoming Feature", TicketType: ticket.TypeFeature, Status: ticket.StatusStarted, Priority: ticket.PriorityHigh, DueDate: &futureDue},
		},
	}

	// 1. User 1 (Creator) creates project
	creatorRouter := setupProjectRouter(1, auth.RoleMember, service, lister)
	createBody, _ := json.Marshal(map[string]any{
		"team_id":          1,
		"name":             "General Platform",
		"prefix":           "ENG",
		"status":           "STARTED",
		"progress_percent": 20,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	creatorRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Add User 2 and User 3 to project
	for _, uid := range []int{2, 3} {
		addBody, _ := json.Marshal(map[string]any{"user_id": uid, "role": "MEMBER"})
		addReq, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/1/members", bytes.NewBuffer(addBody))
		addReq.Header.Set("Content-Type", "application/json")
		addW := httptest.NewRecorder()
		creatorRouter.ServeHTTP(addW, addReq)
		if addW.Code != http.StatusCreated {
			t.Fatalf("expected 201 adding member %d, got %d: %s", uid, addW.Code, addW.Body.String())
		}
	}

	// 3. Non-creator User 2 tries to remove User 3 -> MUST be 403 Forbidden
	nonCreatorRouter := setupProjectRouter(2, auth.RoleMember, service, lister)
	forbidReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/projects/1/members/3", nil)
	forbidW := httptest.NewRecorder()
	nonCreatorRouter.ServeHTTP(forbidW, forbidReq)
	if forbidW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when non-creator removes member, got %d: %s", forbidW.Code, forbidW.Body.String())
	}

	// 4. Creator User 1 removes User 3 -> MUST succeed (200 OK)
	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/projects/1/members/3", nil)
	delW := httptest.NewRecorder()
	creatorRouter.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when creator removes member, got %d: %s", delW.Code, delW.Body.String())
	}

	// 5. Update project status to ON_HOLD and progress_percent to 60
	patchBody, _ := json.Marshal(map[string]any{"status": "ON_HOLD", "progress_percent": 60})
	patchReq, _ := http.NewRequest(http.MethodPatch, "/api/v1/projects/1", bytes.NewBuffer(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchW := httptest.NewRecorder()
	creatorRouter.ServeHTTP(patchW, patchReq)
	if patchW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating project, got %d: %s", patchW.Code, patchW.Body.String())
	}

	// 6. Check Project Timeline
	tlReq, _ := http.NewRequest(http.MethodGet, "/api/v1/projects/1/timeline", nil)
	tlW := httptest.NewRecorder()
	creatorRouter.ServeHTTP(tlW, tlReq)
	if tlW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK getting timeline, got %d", tlW.Code)
	}
	var tlRes struct {
		Data project.TimelineResponse `json:"data"`
	}
	_ = json.Unmarshal(tlW.Body.Bytes(), &tlRes)
	if len(tlRes.Data.Items) != 2 || !tlRes.Data.Items[0].IsOverdue || tlRes.Data.Items[1].IsOverdue {
		t.Fatalf("unexpected timeline items: %+v", tlRes.Data.Items)
	}

	// 7. Check Project Logs
	logReq, _ := http.NewRequest(http.MethodGet, "/api/v1/projects/1/logs", nil)
	logW := httptest.NewRecorder()
	creatorRouter.ServeHTTP(logW, logReq)
	if logW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK getting project logs, got %d", logW.Code)
	}
	var logRes struct {
		Data []project.ProjectLog `json:"data"`
	}
	_ = json.Unmarshal(logW.Body.Bytes(), &logRes)
	if len(logRes.Data) < 4 {
		t.Fatalf("expected at least 4 project logs, got %d", len(logRes.Data))
	}
}

