package collab_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/collab"
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

func setupCollabRouterWithRepo(userRole auth.Role, userID int, repo collab.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	service := collab.NewService(repo)
	mockTicket := &ticket.Ticket{ID: 10, Key: "ENG-10"}
	handler := collab.NewHandler(service, &mockTicketResolver{ticket: mockTicket}, "/tmp/collab_uploads")

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_email", "alice@example.com")
		c.Set("user_role", string(userRole))
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)

	return router
}

func setupCollabRouter(userRole auth.Role, userID int) (*gin.Engine, collab.Repository) {
	userRepo := auth.NewMemoryRepository()
	_, _ = userRepo.CreateUser(context.Background(), &auth.User{
		Email:    "alice@example.com",
		FullName: "Alice Dev",
		Role:     userRole,
	})

	repo := collab.NewMemoryRepository(userRepo)
	router := setupCollabRouterWithRepo(userRole, userID, repo)
	return router, repo
}

func TestMentionExtraction(t *testing.T) {
	body := "Hey @alex and @sarah@company.com, please check this PR!"
	mentions := collab.ExtractMentions(body)

	if len(mentions) != 2 {
		t.Fatalf("expected 2 mentions, got %d: %+v", len(mentions), mentions)
	}

	if mentions[0] != "alex" || mentions[1] != "sarah@company.com" {
		t.Fatalf("unexpected extracted mentions: %+v", mentions)
	}
}

func TestComment_Create_Update_And_Permissions(t *testing.T) {
	// Setup with user ID 1 (Author)
	router, repo := setupCollabRouter(auth.RoleMember, 1)

	// 1. Create Comment on /api/v1/tickets/ENG-10/comments
	commentPayload, _ := json.Marshal(map[string]any{
		"body": "Fixes bug mentioned by @security_team",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-10/comments", bytes.NewBuffer(commentPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on comment, got %d: %s", w.Code, w.Body.String())
	}

	var commentRes struct {
		Data collab.Comment `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &commentRes)
	if len(commentRes.Data.Mentions) != 1 || commentRes.Data.Mentions[0] != "security_team" {
		t.Fatalf("expected parsed mention 'security_team', got %+v", commentRes.Data.Mentions)
	}

	commentID := commentRes.Data.ID

	// 2. Author updates comment
	patchPayload, _ := json.Marshal(map[string]any{
		"body": "Updated comment text by author",
	})
	patchReq, _ := http.NewRequest(http.MethodPatch, "/api/v1/comments/1", bytes.NewBuffer(patchPayload))
	patchReq.Header.Set("Content-Type", "application/json")
	patchW := httptest.NewRecorder()
	router.ServeHTTP(patchW, patchReq)

	if patchW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating own comment, got %d", patchW.Code)
	}

	// 3. Different user (ID 99) attempts to edit -> Forbidden
	otherUserRouter := setupCollabRouterWithRepo(auth.RoleMember, 99, repo)
	forbidReq, _ := http.NewRequest(http.MethodPatch, "/api/v1/comments/1", bytes.NewBuffer(patchPayload))
	forbidReq.Header.Set("Content-Type", "application/json")
	forbidW := httptest.NewRecorder()
	otherUserRouter.ServeHTTP(forbidW, forbidReq)

	if forbidW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-author, got %d", forbidW.Code)
	}

	// 4. Admin (ID 99 with RoleAdmin) CAN delete -> 200 OK
	adminRouter := setupCollabRouterWithRepo(auth.RoleAdmin, 99, repo)
	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/comments/1", nil)
	delW := httptest.NewRecorder()
	adminRouter.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when Admin deletes comment, got %d", delW.Code)
	}

	_ = commentID
}

func TestWatchers_SubscribeAndUnsubscribe(t *testing.T) {
	router, _ := setupCollabRouter(auth.RoleMember, 1)

	// 1. Subscribe to ticket
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-10/watchers", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created adding watcher, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Duplicate subscription -> Conflict
	dupReq, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-10/watchers", bytes.NewBufferString("{}"))
	dupReq.Header.Set("Content-Type", "application/json")
	dupW := httptest.NewRecorder()
	router.ServeHTTP(dupW, dupReq)

	if dupW.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate watcher, got %d", dupW.Code)
	}

	// 3. Unsubscribe -> 200 OK
	unwatchReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/tickets/ENG-10/watchers", nil)
	unwatchW := httptest.NewRecorder()
	router.ServeHTTP(unwatchW, unwatchReq)

	if unwatchW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK removing watcher, got %d", unwatchW.Code)
	}
}

func TestAttachment_StorageProvider(t *testing.T) {
	memStorage := collab.NewMemoryStorage()
	userRepo := auth.NewMemoryRepository()
	repo := collab.NewMemoryRepository(userRepo)
	service := collab.NewService(repo, memStorage)
	mockTicket := &ticket.Ticket{ID: 10, Key: "ENG-10"}
	handler := collab.NewHandler(service, &mockTicketResolver{ticket: mockTicket})

	router := gin.New()
	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	}
	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)

	// Direct service SaveAttachment
	att, err := service.SaveAttachment(
		context.Background(),
		10,
		1,
		"system_log.txt",
		"text/plain",
		12,
		bytes.NewBufferString("hello world!"),
	)
	if err != nil {
		t.Fatalf("failed to save attachment via service: %v", err)
	}

	if att.FileName != "system_log.txt" || att.FileSize != 12 {
		t.Fatalf("unexpected attachment metadata: %+v", att)
	}

	// Verify listing via HTTP
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets/ENG-10/attachments", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing attachments, got %d: %s", w.Code, w.Body.String())
	}
}
