package notification_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ticketing-api/internal/auth"
	"ticketing-api/internal/notification"

	"github.com/gin-gonic/gin"
)

type mockUserResolver struct {
	user *auth.User
}

func (m *mockUserResolver) GetUserByID(ctx context.Context, id int) (*auth.User, error) {
	if m.user != nil {
		return m.user, nil
	}
	return nil, auth.ErrUserNotFound
}

func setupNotificationRouter() (*gin.Engine, *notification.Service) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockUser := &auth.User{
		ID:       2,
		Email:    "sarah@company.com",
		FullName: "Sarah Connor",
		Role:     auth.RoleMember,
	}
	repo := notification.NewMemoryRepository(&mockUserResolver{user: mockUser})
	service := notification.NewService(repo)
	handler := notification.NewHandler(service)

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1) // Authenticated user ID is 1
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)

	return router, service
}

func TestNotifications_Lifecycle(t *testing.T) {
	router, service := setupNotificationRouter()
	ctx := context.Background()

	actorID := 2
	ticketID := 42

	// 1. Generate notification for User 1 from Actor 2
	n1, err := service.Notify(ctx, 1, &actorID, &ticketID, notification.TypeMention, "Sarah mentioned you", "Hey @lead, please review.")
	if err != nil {
		t.Fatalf("failed to create notification: %v", err)
	}
	if n1.ID != 1 || n1.IsRead {
		t.Fatalf("unexpected notification state: %+v", n1)
	}

	// 2. Generate another notification
	_, _ = service.Notify(ctx, 1, &actorID, &ticketID, notification.TypeAssigned, "Ticket Assigned", "You were assigned to ENG-42")

	// 3. Self-notification should be skipped
	selfID := 1
	selfN, err := service.Notify(ctx, 1, &selfID, &ticketID, notification.TypeCommentAdded, "Self Comment", "I commented")
	if err != nil || selfN != nil {
		t.Fatalf("expected self-notification to be skipped, got: %+v, err: %v", selfN, err)
	}

	// 4. List notifications via HTTP GET /api/v1/notifications
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var listRes struct {
		Data []notification.Notification `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listRes)
	if listRes.Meta.Total != 2 {
		t.Fatalf("expected 2 notifications, got %d", listRes.Meta.Total)
	}

	// 5. Mark single notification as read via PATCH /api/v1/notifications/1/read
	readReq, _ := http.NewRequest(http.MethodPatch, "/api/v1/notifications/1/read", nil)
	readW := httptest.NewRecorder()
	router.ServeHTTP(readW, readReq)

	if readW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK marking read, got %d", readW.Code)
	}

	// 6. Filter by unread_only=true
	unreadReq, _ := http.NewRequest(http.MethodGet, "/api/v1/notifications?unread_only=true", nil)
	unreadW := httptest.NewRecorder()
	router.ServeHTTP(unreadW, unreadReq)

	_ = json.Unmarshal(unreadW.Body.Bytes(), &listRes)
	if listRes.Meta.Total != 1 {
		t.Fatalf("expected 1 unread notification remaining, got %d", listRes.Meta.Total)
	}

	// 7. Mark all as read via PATCH /api/v1/notifications/read-all
	allReq, _ := http.NewRequest(http.MethodPatch, "/api/v1/notifications/read-all", nil)
	allW := httptest.NewRecorder()
	router.ServeHTTP(allW, allReq)

	if allW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK marking all read, got %d", allW.Code)
	}

	// 8. Filter unread again -> 0
	unreadReq2, _ := http.NewRequest(http.MethodGet, "/api/v1/notifications?unread_only=true", nil)
	unreadW2 := httptest.NewRecorder()
	router.ServeHTTP(unreadW2, unreadReq2)

	_ = json.Unmarshal(unreadW2.Body.Bytes(), &listRes)
	if listRes.Meta.Total != 0 {
		t.Fatalf("expected 0 unread notifications, got %d", listRes.Meta.Total)
	}
}

