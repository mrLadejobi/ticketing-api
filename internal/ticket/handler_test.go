package ticket_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/middleware"
	"ticketing-api/internal/ticket"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())

	repo := ticket.NewMemoryRepository()
	service := ticket.NewService(repo)
	handler := ticket.NewHandler(service)

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)

	return router
}

func TestCreateTicket_Success(t *testing.T) {
	router := setupTestRouter()

	body := map[string]any{
		"title":       "Fix race condition in ticket assignment",
		"description": "Goroutine leak when updating ticket status concurrently",
		"priority":    "URGENT",
		"assignee":    "alex",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Success bool          `json:"success"`
		Data    ticket.Ticket `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !res.Success || res.Data.ID == 0 || res.Data.Status != ticket.StatusTodo {
		t.Fatalf("unexpected ticket payload: %+v", res)
	}

	// Verify X-Request-ID header propagation
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected X-Request-ID header to be present")
	}
}

func TestCreateTicket_StructuredValidationError(t *testing.T) {
	router := setupTestRouter()

	body := map[string]any{
		"title": "AB", // Min 3 characters
		// Missing description
		"priority": "SUPER_HIGH", // Invalid enum
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}

	var res struct {
		Success bool `json:"success"`
		Error   struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
			Details   []struct {
				Field string `json:"field"`
				Issue string `json:"issue"`
			} `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse validation error response: %v", err)
	}

	if res.Success || res.Error.Code != "INVALID_ARGUMENT" || len(res.Error.Details) == 0 {
		t.Fatalf("expected detailed validation errors, got: %+v", res)
	}
}

func TestListTickets_FilteringAndPagination(t *testing.T) {
	router := setupTestRouter()

	// Seed 2 tickets
	for _, title := range []string{"Ticket Alpha", "Ticket Beta"} {
		body, _ := json.Marshal(map[string]any{
			"title":       title,
			"description": "Description for " + title,
			"priority":    "MEDIUM",
			"assignee":    "sam",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// Filter by search query
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets?search=Alpha&limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var res struct {
		Success bool            `json:"success"`
		Data    []ticket.Ticket `json:"data"`
		Meta    struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}

	if res.Meta.Total != 1 || len(res.Data) != 1 || res.Data[0].Title != "Ticket Alpha" {
		t.Fatalf("expected 1 filtered ticket 'Ticket Alpha', got: %+v", res)
	}
}
