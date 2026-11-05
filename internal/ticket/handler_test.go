package ticket_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ticketing-api/internal/auth"
	"ticketing-api/internal/middleware"
	"ticketing-api/internal/ticket"

	"github.com/gin-gonic/gin"
)

type mockKeyGen struct {
	counter int
}

func (m *mockKeyGen) NextIssueKey(ctx context.Context, projectID int) (string, error) {
	m.counter++
	return fmt.Sprintf("ENG-%d", m.counter), nil
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())

	userRepo := auth.NewMemoryRepository()
	_, _ = userRepo.CreateUser(context.Background(), &auth.User{
		Email:    "lead@example.com",
		FullName: "Lead Dev",
		Role:     auth.RoleAdmin,
	})

	repo := ticket.NewMemoryRepository(userRepo)
	keyGen := &mockKeyGen{}
	service := ticket.NewService(repo, keyGen)
	handler := ticket.NewHandler(service)

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("user_email", "lead@example.com")
		c.Set("user_role", string(auth.RoleAdmin))
		c.Next()
	}
	dummyAdmin := func(c *gin.Context) {
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth, dummyAdmin)

	return router
}

func TestTicket_Lifecycle_Keys_And_OptimisticLocking(t *testing.T) {
	router := setupTestRouter()

	// 1. Create Ticket -> Should generate key ENG-1
	createBody, _ := json.Marshal(map[string]any{
		"project_id":  1,
		"title":       "Refactor Auth Middleware",
		"description": "Enterprise RBAC and Optimistic Locking",
		"priority":    "URGENT",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets", bytes.NewBuffer(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var createRes struct {
		Data ticket.Ticket `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &createRes)
	if createRes.Data.Key != "ENG-1" || createRes.Data.Version != 1 {
		t.Fatalf("expected key ENG-1 and version 1, got %+v", createRes.Data)
	}

	// 2. Query by Human-Readable Key: GET /api/v1/tickets/ENG-1
	reqKey, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets/ENG-1", nil)
	wKey := httptest.NewRecorder()
	router.ServeHTTP(wKey, reqKey)

	if wKey.Code != http.StatusOK {
		t.Fatalf("expected 200 OK querying by key ENG-1, got %d", wKey.Code)
	}

	// 3. Test Optimistic Locking: Attempt PATCH with stale version
	staleVersion := 99
	patchStale, _ := json.Marshal(map[string]any{
		"title":   "New Title",
		"version": staleVersion,
	})
	reqStale, _ := http.NewRequest(http.MethodPatch, "/api/v1/tickets/ENG-1", bytes.NewBuffer(patchStale))
	reqStale.Header.Set("Content-Type", "application/json")
	wStale := httptest.NewRecorder()
	router.ServeHTTP(wStale, reqStale)

	if wStale.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on stale version, got %d", wStale.Code)
	}

	// 4. Test Valid PATCH with correct version -> Increments version to 2
	validVersion := 1
	newStatus := ticket.StatusInReview
	patchValid, _ := json.Marshal(map[string]any{
		"status":  newStatus,
		"version": validVersion,
	})
	reqValid, _ := http.NewRequest(http.MethodPatch, "/api/v1/tickets/ENG-1", bytes.NewBuffer(patchValid))
	reqValid.Header.Set("Content-Type", "application/json")
	wValid := httptest.NewRecorder()
	router.ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid update, got %d: %s", wValid.Code, wValid.Body.String())
	}

	var updateRes struct {
		Data ticket.Ticket `json:"data"`
	}
	_ = json.Unmarshal(wValid.Body.Bytes(), &updateRes)
	if updateRes.Data.Status != ticket.StatusInReview || updateRes.Data.Version != 2 {
		t.Fatalf("expected status IN_REVIEW and version 2, got %+v", updateRes.Data)
	}

	// 5. Test Soft Delete
	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/tickets/ENG-1", nil)
	delW := httptest.NewRecorder()
	router.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on soft delete, got %d", delW.Code)
	}

	// 6. Test Restore (Admin)
	restoreReq, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-1/restore", nil)
	resW := httptest.NewRecorder()
	router.ServeHTTP(resW, restoreReq)
	if resW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on restore, got %d: %s", resW.Code, resW.Body.String())
	}
}

func TestTicket_BulkUpdate(t *testing.T) {
	router := setupTestRouter()

	// 1. Create two tickets
	for i := 1; i <= 2; i++ {
		createBody, _ := json.Marshal(map[string]any{
			"project_id":  1,
			"title":       fmt.Sprintf("Issue %d", i),
			"description": "Desc",
			"priority":    "LOW",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets", bytes.NewBuffer(createBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created on ticket %d, got %d", i, w.Code)
		}
	}

	// 2. Bulk update both tickets to IN_PROGRESS and HIGH priority
	bulkBody, _ := json.Marshal(map[string]any{
		"ticket_ids": []int{1, 2},
		"status":     "IN_PROGRESS",
		"priority":   "HIGH",
	})
	bulkReq, _ := http.NewRequest(http.MethodPatch, "/api/v1/tickets/bulk", bytes.NewBuffer(bulkBody))
	bulkReq.Header.Set("Content-Type", "application/json")
	bulkW := httptest.NewRecorder()
	router.ServeHTTP(bulkW, bulkReq)

	if bulkW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on bulk update, got %d: %s", bulkW.Code, bulkW.Body.String())
	}

	var bulkRes struct {
		Data struct {
			UpdatedCount int             `json:"updated_count"`
			Tickets      []ticket.Ticket `json:"tickets"`
		} `json:"data"`
	}
	_ = json.Unmarshal(bulkW.Body.Bytes(), &bulkRes)
	if bulkRes.Data.UpdatedCount != 2 {
		t.Fatalf("expected 2 tickets updated, got %d", bulkRes.Data.UpdatedCount)
	}
	if bulkRes.Data.Tickets[0].Status != ticket.StatusInProgress || bulkRes.Data.Tickets[0].Priority != ticket.PriorityHigh {
		t.Fatalf("expected tickets updated to IN_PROGRESS and HIGH, got %+v", bulkRes.Data.Tickets[0])
	}
}

type mockActivityTracker struct {
	activities []string
	failOnCall bool
}

func (m *mockActivityTracker) RecordTicketActivity(ctx context.Context, ticketID int, actorID *int, field, oldValue, newValue string) error {
	if m.failOnCall {
		return fmt.Errorf("activity recording database failure")
	}
	m.activities = append(m.activities, fmt.Sprintf("%d:%s:%s->%s", ticketID, field, oldValue, newValue))
	return nil
}

func TestTicket_ActivityTransactionalIntegrity(t *testing.T) {
	userRepo := auth.NewMemoryRepository()
	repo := ticket.NewMemoryRepository(userRepo)
	keyGen := &mockKeyGen{}
	service := ticket.NewService(repo, keyGen)

	tracker := &mockActivityTracker{}
	service.SetActivityRecorder(tracker)

	created, err := service.Create(context.Background(), 1, ticket.CreateTicketRequest{
		ProjectID:   1,
		Title:       "Transactional Audit Ticket",
		Description: "Testing atomic logging",
		Priority:    ticket.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// 1. Failed ticket update (e.g. non-existent ticket 999) must NOT record any activity
	inProg := ticket.StatusInProgress
	_, err = service.Update(context.Background(), 999, ticket.UpdateTicketRequest{
		Status: &inProg,
	})
	if err == nil {
		t.Fatalf("expected error updating non-existent ticket, got nil")
	}
	if len(tracker.activities) != 0 {
		t.Fatalf("expected 0 orphan activities after failed update, got %d", len(tracker.activities))
	}

	// 2. Successful update records activity
	updated, err := service.Update(context.Background(), created.ID, ticket.UpdateTicketRequest{
		Status: &inProg,
	})
	if err != nil {
		t.Fatalf("expected successful update, got %v", err)
	}
	if updated.Status != ticket.StatusInProgress {
		t.Fatalf("expected status IN_PROGRESS, got %s", updated.Status)
	}
	if len(tracker.activities) != 1 {
		t.Fatalf("expected exactly 1 activity recorded, got %d", len(tracker.activities))
	}

	// 3. If ActivityRecorder fails, the update call must fail and report the error
	tracker.failOnCall = true
	done := ticket.StatusDone
	_, err = service.Update(context.Background(), created.ID, ticket.UpdateTicketRequest{
		Status: &done,
	})
	if err == nil {
		t.Fatalf("expected update to fail when activity recording fails, got nil")
	}
}
