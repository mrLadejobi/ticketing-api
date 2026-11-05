package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ticketing-api/internal/auth"
	"ticketing-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func setupAuthRouter() (*gin.Engine, *auth.TokenManager) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	tm := auth.NewTokenManager("test-jwt-secret-key-32-bytes-long!", 15*time.Minute, 7*24*time.Hour)
	repo := auth.NewMemoryRepository()
	service := auth.NewService(repo, tm)
	handler := auth.NewHandler(service, false)

	authMiddleware := middleware.Authenticate(tm)
	adminMiddleware := middleware.RequireRoles(auth.RoleAdmin)

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, authMiddleware, adminMiddleware)

	return router, tm
}

func TestRegister_Login_And_RefreshToken_Flow(t *testing.T) {
	router, _ := setupAuthRouter()

	// 1. Register
	regPayload := map[string]any{
		"full_name": "Bob Architect",
		"email":     "bob@example.com",
		"password":  "SecurePassword99!",
	}
	body, _ := json.Marshal(regPayload)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on register, got %d: %s", w.Code, w.Body.String())
	}

	// Verify both access_token and refresh_token HttpOnly cookies are set
	cookies := w.Result().Cookies()
	var accessCookie, refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			accessCookie = c
		} else if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}

	if accessCookie == nil || !accessCookie.HttpOnly {
		t.Fatalf("expected HttpOnly access_token cookie")
	}
	if refreshCookie == nil || !refreshCookie.HttpOnly {
		t.Fatalf("expected HttpOnly refresh_token cookie")
	}

	// 2. Call /refresh using the refresh_token cookie
	refReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refReq.AddCookie(refreshCookie)
	refRec := httptest.NewRecorder()

	router.ServeHTTP(refRec, refReq)

	if refRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /refresh, got %d: %s", refRec.Code, refRec.Body.String())
	}

	// Verify that /refresh rotated the cookies
	refreshedCookies := refRec.Result().Cookies()
	var newAccessCookie *http.Cookie
	for _, c := range refreshedCookies {
		if c.Name == "access_token" {
			newAccessCookie = c
		}
	}
	if newAccessCookie == nil || newAccessCookie.Value == "" {
		t.Fatalf("expected new access_token cookie on refresh")
	}

	// 3. Test Logout clears cookies
	logoutReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /logout, got %d", logoutRec.Code)
	}

	for _, c := range logoutRec.Result().Cookies() {
		if c.MaxAge > 0 {
			t.Fatalf("expected cookie %s to be cleared with MaxAge <= 0, got %d", c.Name, c.MaxAge)
		}
	}
}
