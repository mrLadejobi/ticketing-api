package cycle_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/cycle"
	"ticketing-api/internal/project"
	"ticketing-api/internal/ticket"
)

type mockProjectResolver struct {
	projects map[int]*project.Project
}

func (m *mockProjectResolver) GetProjectByID(ctx context.Context, id int) (*project.Project, error) {
	if p, exists := m.projects[id]; exists {
		return p, nil
	}
	return nil, project.ErrProjectNotFound
}

type mockTicketService struct {
	tickets map[int]*ticket.Ticket
}

func (m *mockTicketService) BulkUpdate(ctx context.Context, req ticket.BulkUpdateTicketsRequest) ([]ticket.Ticket, error) {
	var updated []ticket.Ticket
	for _, id := range req.TicketIDs {
		if t, exists := m.tickets[id]; exists {
			if req.Status != nil {
				t.Status = *req.Status
			}
			if req.Priority != nil {
				t.Priority = *req.Priority
			}
			if req.CycleID != nil {
				t.CycleID = req.CycleID
			}
			updated = append(updated, *t)
		}
	}
	return updated, nil
}

func (m *mockTicketService) ListByCycle(ctx context.Context, cycleID int) ([]ticket.Ticket, error) {
	var list []ticket.Ticket
	for _, t := range m.tickets {
		if t.CycleID != nil && *t.CycleID == cycleID {
			list = append(list, *t)
		}
	}
	return list, nil
}

func setupCycleTestRouter() (*gin.Engine, *cycle.Service, *mockTicketService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	projResolver := &mockProjectResolver{
		projects: map[int]*project.Project{
			1: {ID: 1, Name: "Core Engine", Prefix: "ENG"},
		},
	}

	cid1 := 1
	ticketService := &mockTicketService{
		tickets: map[int]*ticket.Ticket{
			1: {ID: 1, Key: "ENG-1", Title: "Setup Kafka", Status: ticket.StatusDone, Priority: ticket.PriorityHigh, CycleID: &cid1},
			2: {ID: 2, Key: "ENG-2", Title: "Redis Cache", Status: ticket.StatusInProgress, Priority: ticket.PriorityMedium, CycleID: &cid1},
			3: {ID: 3, Key: "ENG-3", Title: "Telemetry", Status: ticket.StatusTodo, Priority: ticket.PriorityLow},
		},
	}

	repo := cycle.NewMemoryRepository()
	service := cycle.NewService(repo, ticketService)
	handler := cycle.NewHandler(service, projResolver)

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)

	return router, service, ticketService
}

func TestCycle_Create_And_List(t *testing.T) {
	router, _, _ := setupCycleTestRouter()

	now := time.Now().UTC()
	start := now
	end := now.Add(14 * 24 * time.Hour)

	// 1. Create Cycle
	payload, _ := json.Marshal(map[string]any{
		"name":       "Sprint 24 - High Throughput",
		"goal":       "Deploy distributed caching",
		"start_date": start.Format(time.RFC3339),
		"end_date":   end.Format(time.RFC3339),
		"status":     "ACTIVE",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/1/cycles", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on cycle, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Reject Invalid Date Range (start_date > end_date)
	badPayload, _ := json.Marshal(map[string]any{
		"name":       "Sprint 25",
		"start_date": end.Format(time.RFC3339),
		"end_date":   start.Format(time.RFC3339),
	})
	badReq, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/1/cycles", bytes.NewBuffer(badPayload))
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	router.ServeHTTP(badW, badReq)

	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid date order, got %d", badW.Code)
	}

	// 3. List Project Cycles
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/projects/1/cycles", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing cycles, got %d", listW.Code)
	}
}

func TestCycle_AssignTickets_And_Analytics(t *testing.T) {
	router, service, _ := setupCycleTestRouter()

	now := time.Now().UTC()
	c, err := service.Create(context.Background(), 1, cycle.CreateCycleRequest{
		Name:      "Sprint 1",
		Goal:      "MVP Delivery",
		StartDate: now,
		EndDate:   now.Add(7 * 24 * time.Hour),
		Status:    cycle.StatusActive,
	})
	if err != nil {
		t.Fatalf("failed to create cycle: %v", err)
	}

	// 1. Assign ticket 3 to cycle 1
	assignPayload, _ := json.Marshal(map[string]any{
		"ticket_ids": []int{3},
	})
	assignReq, _ := http.NewRequest(http.MethodPost, "/api/v1/cycles/1/tickets", bytes.NewBuffer(assignPayload))
	assignReq.Header.Set("Content-Type", "application/json")
	assignW := httptest.NewRecorder()
	router.ServeHTTP(assignW, assignReq)

	if assignW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK assigning tickets, got %d: %s", assignW.Code, assignW.Body.String())
	}

	// 2. Fetch Analytics
	analyticsReq, _ := http.NewRequest(http.MethodGet, "/api/v1/cycles/1/analytics", nil)
	analyticsW := httptest.NewRecorder()
	router.ServeHTTP(analyticsW, analyticsReq)

	if analyticsW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK getting analytics, got %d: %s", analyticsW.Code, analyticsW.Body.String())
	}

	var res struct {
		Data cycle.CycleAnalytics `json:"data"`
	}
	_ = json.Unmarshal(analyticsW.Body.Bytes(), &res)

	// In cycle 1, tickets 1, 2, 3 are now present:
	// Ticket 1: DONE
	// Ticket 2: IN_PROGRESS
	// Ticket 3: TODO
	// Total: 3, Completed: 1 (33.33%)
	if res.Data.TotalTickets != 3 {
		t.Fatalf("expected 3 total tickets in cycle analytics, got %d", res.Data.TotalTickets)
	}
	if res.Data.CompletedTickets != 1 {
		t.Fatalf("expected 1 completed ticket, got %d", res.Data.CompletedTickets)
	}
	if res.Data.CompletionPercentage != 33.33 {
		t.Fatalf("expected 33.33%% completion, got %.2f%%", res.Data.CompletionPercentage)
	}
	if res.Data.StatusBreakdown[ticket.StatusDone] != 1 || res.Data.StatusBreakdown[ticket.StatusInProgress] != 1 {
		t.Fatalf("unexpected status breakdown: %+v", res.Data.StatusBreakdown)
	}

	_ = c
}

