package relation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/relation"
	"ticketing-api/internal/ticket"
)

type mockTicketResolver struct {
	tickets map[string]*ticket.Ticket
}

func (m *mockTicketResolver) GetByIDOrKey(ctx context.Context, idOrKey string) (*ticket.Ticket, error) {
	if t, exists := m.tickets[idOrKey]; exists {
		return t, nil
	}
	return nil, ticket.ErrNotFound
}

func setupTestRouter() (*gin.Engine, *relation.Service, *mockTicketResolver) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	tickets := map[string]*ticket.Ticket{
		"1":      {ID: 1, Key: "ENG-1", Title: "Database migration", Status: ticket.StatusInProgress, Priority: ticket.PriorityHigh},
		"ENG-1":  {ID: 1, Key: "ENG-1", Title: "Database migration", Status: ticket.StatusInProgress, Priority: ticket.PriorityHigh},
		"2":      {ID: 2, Key: "ENG-2", Title: "Deploy backend", Status: ticket.StatusTodo, Priority: ticket.PriorityMedium},
		"ENG-2":  {ID: 2, Key: "ENG-2", Title: "Deploy backend", Status: ticket.StatusTodo, Priority: ticket.PriorityMedium},
	}
	resolver := &mockTicketResolver{tickets: tickets}

	repo := relation.NewMemoryRepository(resolver)
	service := relation.NewService(repo)
	handler := relation.NewHandler(service, resolver)

	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	}

	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1, dummyAuth)

	return router, service, resolver
}

func TestRelation_Create_And_List(t *testing.T) {
	router, _, _ := setupTestRouter()

	// 1. Create BLOCKS relation between ENG-1 and ENG-2
	payload, _ := json.Marshal(map[string]any{
		"target_ticket_id_or_key": "ENG-2",
		"relation_type":           "BLOCKS",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-1/relations", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Data relation.TicketRelation `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Data.SourceTicketID != 1 || res.Data.TargetTicketID != 2 || res.Data.RelationType != relation.RelationBlocks {
		t.Fatalf("unexpected relation data: %+v", res.Data)
	}

	// 2. Prevent Duplicate Relation
	dupReq, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-1/relations", bytes.NewBuffer(payload))
	dupReq.Header.Set("Content-Type", "application/json")
	dupW := httptest.NewRecorder()
	router.ServeHTTP(dupW, dupReq)

	if dupW.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate relation, got %d", dupW.Code)
	}

	// 3. List relations for ENG-2 (target ticket should also see it)
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets/ENG-2/relations", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing relations, got %d", listW.Code)
	}

	var listRes struct {
		Data []relation.TicketRelation `json:"data"`
	}
	_ = json.Unmarshal(listW.Body.Bytes(), &listRes)
	if len(listRes.Data) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(listRes.Data))
	}
}

func TestRelation_SelfRelationRejected(t *testing.T) {
	router, _, _ := setupTestRouter()

	payload, _ := json.Marshal(map[string]any{
		"target_ticket_id_or_key": "ENG-1",
		"relation_type":           "BLOCKS",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-1/relations", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on self relation, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRelation_Delete(t *testing.T) {
	router, _, _ := setupTestRouter()

	// Create relation first
	payload, _ := json.Marshal(map[string]any{
		"target_ticket_id_or_key": "ENG-2",
		"relation_type":           "RELATES_TO",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets/ENG-1/relations", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Delete relation
	delReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/tickets/ENG-1/relations/1", nil)
	delW := httptest.NewRecorder()
	router.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting relation, got %d", delW.Code)
	}

	// Delete non-existent relation -> 404
	delReq2, _ := http.NewRequest(http.MethodDelete, "/api/v1/tickets/ENG-1/relations/999", nil)
	delW2 := httptest.NewRecorder()
	router.ServeHTTP(delW2, delReq2)

	if delW2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", delW2.Code)
	}
}

