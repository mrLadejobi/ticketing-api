package label_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/label"
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

type mockTicketResolver struct {
	tickets map[string]*ticket.Ticket
}

func (m *mockTicketResolver) GetByIDOrKey(ctx context.Context, idOrKey string) (*ticket.Ticket, error) {
	if t, exists := m.tickets[idOrKey]; exists {
		return t, nil
	}
	return nil, ticket.ErrNotFound
}

func setupLabelTestRouter() (*gin.Engine, *label.Service) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	projResolver := &mockProjectResolver{
		projects: map[int]*project.Project{
			1: {ID: 1, Name: "Frontend App", Prefix: "FE"},
		},
	}

	ticketResolver := &mockTicketResolver{
		tickets: map[string]*ticket.Ticket{
			"1":     {ID: 1, Key: "FE-1", Title: "Fix CSS"},
			"FE-1": {ID: 1, Key: "FE-1", Title: "Fix CSS"},
		},
	}

	repo := label.NewMemoryRepository()
	service := label.NewService(repo)
	handler := label.NewHandler(service, projResolver, ticketResolver)

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)

	return router, service
}

func TestProjectLabels_Create_And_List(t *testing.T) {
	router, _ := setupLabelTestRouter()

	// 1. Create label "bug"
	payload, _ := json.Marshal(map[string]any{
		"name":      "bug",
		"color_hex": "#EF4444",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/1/labels", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Reject duplicate label name in same project
	dupReq, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/1/labels", bytes.NewBuffer(payload))
	dupReq.Header.Set("Content-Type", "application/json")
	dupW := httptest.NewRecorder()
	router.ServeHTTP(dupW, dupReq)

	if dupW.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate label name, got %d", dupW.Code)
	}

	// 3. Reject invalid hex color
	badColorPayload, _ := json.Marshal(map[string]any{
		"name":      "feature",
		"color_hex": "not-a-color",
	})
	badReq, _ := http.NewRequest(http.MethodPost, "/api/v1/projects/1/labels", bytes.NewBuffer(badColorPayload))
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	router.ServeHTTP(badW, badReq)

	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid color, got %d", badW.Code)
	}

	// 4. List Project Labels
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/projects/1/labels", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing labels, got %d", listW.Code)
	}
}

func TestTicketLabels_Add_List_And_Remove(t *testing.T) {
	router, service := setupLabelTestRouter()

	// Create label directly via service
	l, err := service.CreateProjectLabel(context.Background(), 1, "backend", "#10B981")
	if err != nil {
		t.Fatalf("failed to create label: %v", err)
	}

	// 1. Attach label to ticket FE-1
	addPayload, _ := json.Marshal(map[string]any{
		"label_id": l.ID,
	})
	addReq, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/FE-1/labels", bytes.NewBuffer(addPayload))
	addReq.Header.Set("Content-Type", "application/json")
	addW := httptest.NewRecorder()
	router.ServeHTTP(addW, addReq)

	if addW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created attaching label, got %d: %s", addW.Code, addW.Body.String())
	}

	// 2. Reject duplicate attachment
	dupReq, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/FE-1/labels", bytes.NewBuffer(addPayload))
	dupReq.Header.Set("Content-Type", "application/json")
	dupW := httptest.NewRecorder()
	router.ServeHTTP(dupW, dupReq)

	if dupW.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate label attachment, got %d", dupW.Code)
	}

	// 3. List ticket labels
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets/FE-1/labels", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing ticket labels, got %d", listW.Code)
	}

	// 4. Remove label from ticket
	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/tickets/FE-1/labels/1", nil)
	delW := httptest.NewRecorder()
	router.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK removing label, got %d: %s", delW.Code, delW.Body.String())
	}
}
