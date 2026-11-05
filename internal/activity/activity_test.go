package activity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/activity"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/ticket"
)

type mockTicketResolver struct {
	ticket *ticket.Ticket
}

func (m *mockTicketResolver) GetByIDOrKey(ctx context.Context, idOrKey string) (*ticket.Ticket, error) {
	if m.ticket != nil {
		return m.ticket, nil
	}
	return nil, ticket.ErrNotFound
}

func setupActivityRouter() (*gin.Engine, *activity.Service) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	userRepo := auth.NewMemoryRepository()
	_, _ = userRepo.CreateUser(context.Background(), &auth.User{
		Email:    "admin@example.com",
		FullName: "System Admin",
		Role:     auth.RoleAdmin,
	})

	repo := activity.NewMemoryRepository(userRepo)
	service := activity.NewService(repo)
	handler := activity.NewHandler(service, &mockTicketResolver{ticket: &ticket.Ticket{ID: 1, Key: "ENG-1"}})

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	}
	dummyAdmin := func(c *gin.Context) {
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth, dummyAdmin)

	return router, service
}

func TestActivity_RecordAndList(t *testing.T) {
	router, service := setupActivityRouter()

	ctx := context.Background()
	actorID := 1
	_, err := service.RecordTicketActivity(ctx, 1, &actorID, "status", "TODO", "IN_PROGRESS")
	if err != nil {
		t.Fatalf("failed to record activity: %v", err)
	}
	_, err = service.RecordTicketActivity(ctx, 1, &actorID, "priority", "LOW", "HIGH")
	if err != nil {
		t.Fatalf("failed to record activity: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets/ENG-1/activity?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing activities, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuditLog_RecordAndList(t *testing.T) {
	router, service := setupActivityRouter()

	ctx := context.Background()
	actorID := 1
	_, err := service.RecordAuditLog(ctx, &actorID, "UPDATE_USER_ROLE", "USER", "2", `{"role":"ADMIN"}`, "127.0.0.1")
	if err != nil {
		t.Fatalf("failed to record audit log: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing audit logs, got %d: %s", w.Code, w.Body.String())
	}
}

