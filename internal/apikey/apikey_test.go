package apikey_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ticketing-api/internal/apikey"
	"ticketing-api/internal/auth"

	"github.com/gin-gonic/gin"
)

type mockUserGetter struct {
	user *auth.User
}

func (m *mockUserGetter) GetUserByID(ctx context.Context, id int) (*auth.User, error) {
	if m.user != nil {
		return m.user, nil
	}
	return nil, auth.ErrUserNotFound
}

func setupAPIKeyRouter() (*gin.Engine, *apikey.Service, *mockUserGetter) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockUser := &auth.User{
		ID:       1,
		Email:    "dev@company.com",
		FullName: "Dev User",
		Role:     auth.RoleMember,
	}
	userGetter := &mockUserGetter{user: mockUser}

	repo := apikey.NewMemoryRepository()
	service := apikey.NewService(repo, userGetter)
	handler := apikey.NewHandler(service)

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)

	return router, service, userGetter
}

func TestAPIKey_Create_Validate_And_Delete(t *testing.T) {
	router, service, _ := setupAPIKeyRouter()

	// 1. Create API Key
	payload, _ := json.Marshal(map[string]any{
		"name": "CI/CD Pipeline Runner",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created creating api key, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Data apikey.CreateAPIKeyResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	rawToken := res.Data.Token
	if len(rawToken) < 10 || rawToken[:4] != "pat_" {
		t.Fatalf("expected pat_ token format, got %s", rawToken)
	}

	// 2. Validate token
	user, err := service.ValidateKey(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("failed to validate api key: %v", err)
	}
	if user.ID != 1 || user.Email != "dev@company.com" {
		t.Fatalf("unexpected user retrieved: %+v", user)
	}

	// 3. List API Keys
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing api keys, got %d", listW.Code)
	}

	// 4. Delete API Key
	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/api-keys/1", nil)
	delW := httptest.NewRecorder()
	router.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting api key, got %d", delW.Code)
	}

	// 5. Validating revoked key should fail
	_, err = service.ValidateKey(context.Background(), rawToken)
	if err == nil {
		t.Fatalf("expected error validating revoked key, got nil")
	}
}

func TestAPIKey_ExpiredKey(t *testing.T) {
	_, service, _ := setupAPIKeyRouter()

	past := time.Now().UTC().Add(-1 * time.Hour)
	resp, err := service.CreateKey(context.Background(), 1, apikey.CreateAPIKeyRequest{
		Name:      "Expired Key",
		ExpiresAt: &past,
	})
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	_, err = service.ValidateKey(context.Background(), resp.Token)
	if err != apikey.ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

type spyRepo struct {
	apikey.Repository
	updateCalls atomic.Int32
}

func (s *spyRepo) UpdateLastUsed(ctx context.Context, id int) error {
	s.updateCalls.Add(1)
	return s.Repository.UpdateLastUsed(ctx, id)
}

func TestAPIKey_ThrottledLastUsedAt(t *testing.T) {
	memRepo := apikey.NewMemoryRepository()
	spy := &spyRepo{Repository: memRepo}

	mockUser := &auth.User{
		ID:       1,
		Email:    "dev@company.com",
		FullName: "Dev User",
		Role:     auth.RoleMember,
	}
	userGetter := &mockUserGetter{user: mockUser}
	service := apikey.NewService(spy, userGetter)

	resp, err := service.CreateKey(context.Background(), 1, apikey.CreateAPIKeyRequest{
		Name: "Throttled Key",
	})
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// First call should trigger background update
	_, err = service.ValidateKey(context.Background(), resp.Token)
	if err != nil {
		t.Fatalf("first validation failed: %v", err)
	}

	// Give the goroutine a small window to run
	time.Sleep(50 * time.Millisecond)

	if spy.updateCalls.Load() != 1 {
		t.Fatalf("expected 1 update call after initial validation, got %d", spy.updateCalls.Load())
	}

	// Rapid successive calls should be throttled and NOT call UpdateLastUsed again
	for i := 0; i < 5; i++ {
		_, err = service.ValidateKey(context.Background(), resp.Token)
		if err != nil {
			t.Fatalf("consecutive validation %d failed: %v", i, err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	if spy.updateCalls.Load() != 1 {
		t.Fatalf("expected still exactly 1 update call due to throttling, got %d", spy.updateCalls.Load())
	}
}
