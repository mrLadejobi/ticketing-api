package webhook_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/ticket"
	"ticketing-api/internal/webhook"
)

type mockTicketService struct {
	tickets map[string]*ticket.Ticket
}

func (m *mockTicketService) GetByIDOrKey(c context.Context, idOrKey string) (*ticket.Ticket, error) {
	if t, exists := m.tickets[idOrKey]; exists {
		return t, nil
	}
	return nil, ticket.ErrNotFound
}

func (m *mockTicketService) Update(c context.Context, id int, req ticket.UpdateTicketRequest) (*ticket.Ticket, error) {
	for _, t := range m.tickets {
		if t.ID == id {
			if req.Status != nil {
				t.Status = *req.Status
			}
			return t, nil
		}
	}
	return nil, ticket.ErrNotFound
}

type mockCommentService struct {
	comments []string
}

func (m *mockCommentService) AddSystemComment(c context.Context, ticketID int, body string) error {
	m.comments = append(m.comments, body)
	return nil
}

func computeSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGitWebhook_PR_Opened_And_Merged_AutoTransitions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	secret := "test-secret"
	ticketMap := map[string]*ticket.Ticket{
		"PAY-12": {ID: 12, Key: "PAY-12", Title: "Integrate Stripe", Status: ticket.StatusTodo},
	}
	ticketService := &mockTicketService{tickets: ticketMap}
	commentService := &mockCommentService{}

	handler := webhook.NewHandler(secret, ticketService, commentService)
	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)

	// 1. PR Opened -> Auto transition to IN_REVIEW
	prOpenedPayload, _ := json.Marshal(map[string]any{
		"action": "opened",
		"pull_request": map[string]any{
			"number":   42,
			"title":    "PAY-12: Stripe Webhook handler",
			"body":     "Closes PAY-12. Please review.",
			"html_url": "https://github.com/org/repo/pull/42",
			"merged":   false,
			"head": map[string]any{
				"ref": "feature/PAY-12-stripe",
			},
		},
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/git", bytes.NewBuffer(prOpenedPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", computeSignature(secret, prOpenedPayload))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PR opened webhook, got %d: %s", w.Code, w.Body.String())
	}
	if ticketMap["PAY-12"].Status != ticket.StatusInReview {
		t.Fatalf("expected PAY-12 to be transitioned to IN_REVIEW, got %s", ticketMap["PAY-12"].Status)
	}

	// 2. PR Merged -> Auto transition to DONE
	prMergedPayload, _ := json.Marshal(map[string]any{
		"action": "closed",
		"pull_request": map[string]any{
			"number":   42,
			"title":    "PAY-12: Stripe Webhook handler",
			"html_url": "https://github.com/org/repo/pull/42",
			"merged":   true,
			"head": map[string]any{
				"ref": "feature/PAY-12-stripe",
			},
		},
	})

	mergeReq, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/git", bytes.NewBuffer(prMergedPayload))
	mergeReq.Header.Set("Content-Type", "application/json")
	mergeReq.Header.Set("X-Hub-Signature-256", computeSignature(secret, prMergedPayload))
	mergeW := httptest.NewRecorder()
	router.ServeHTTP(mergeW, mergeReq)

	if mergeW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PR merged webhook, got %d", mergeW.Code)
	}
	if ticketMap["PAY-12"].Status != ticket.StatusDone {
		t.Fatalf("expected PAY-12 to be transitioned to DONE, got %s", ticketMap["PAY-12"].Status)
	}
}

func TestGitWebhook_InvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := webhook.NewHandler("secret", nil, nil)
	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)

	payload := []byte(`{"action":"opened"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/git", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on invalid signature, got %d", w.Code)
	}
}

func TestGitWebhook_DeliveryIdempotency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	ticketMap := map[string]*ticket.Ticket{
		"ENG-99": {
			ID:     99,
			Key:    "ENG-99",
			Status: ticket.StatusTodo,
		},
	}
	mockTickets := &mockTicketService{tickets: ticketMap}
	mockComments := &mockCommentService{}
	tracker := webhook.NewMemoryDeliveryTracker()

	handler := webhook.NewHandler("", mockTickets, mockComments, tracker)
	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)

	payload, _ := json.Marshal(map[string]any{
		"action": "opened",
		"pull_request": map[string]any{
			"number":   99,
			"title":    "ENG-99: Add idempotency tests",
			"html_url": "https://github.com/org/repo/pull/99",
			"merged":   false,
			"head": map[string]any{
				"ref": "feat/ENG-99",
			},
		},
	})

	deliveryID := "delivery-uuid-12345"

	// 1. First delivery should succeed and transition ticket
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/git", bytes.NewBuffer(payload))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-GitHub-Delivery", deliveryID)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on first delivery, got %d: %s", w1.Code, w1.Body.String())
	}
	if ticketMap["ENG-99"].Status != ticket.StatusInReview {
		t.Fatalf("expected ENG-99 transitioned to IN_REVIEW, got %s", ticketMap["ENG-99"].Status)
	}
	if len(mockComments.comments) != 1 {
		t.Fatalf("expected 1 comment created, got %d", len(mockComments.comments))
	}

	// 2. Duplicate delivery with the same X-GitHub-Delivery header must return duplicate_ignored
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/git", bytes.NewBuffer(payload))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-GitHub-Delivery", deliveryID)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on duplicate delivery, got %d", w2.Code)
	}

	var dupRes struct {
		Data struct {
			Status     string `json:"status"`
			DeliveryID string `json:"delivery_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &dupRes)
	if dupRes.Data.Status != "duplicate_ignored" {
		t.Fatalf("expected status duplicate_ignored, got %s", dupRes.Data.Status)
	}

	// Ensure no duplicate comments were added
	if len(mockComments.comments) != 1 {
		t.Fatalf("expected still exactly 1 comment, but got %d", len(mockComments.comments))
	}

	// 3. Duplicate delivery without headers using body SHA-256 fallback
	req3, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/git", bytes.NewBuffer(payload))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on payload with fallback hash, got %d", w3.Code)
	}

	// Immediate replay of exact same payload without headers
	req4, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/git", bytes.NewBuffer(payload))
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)

	var fallbackRes struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w4.Body.Bytes(), &fallbackRes)
	if fallbackRes.Data.Status != "duplicate_ignored" {
		t.Fatalf("expected fallback duplicate_ignored, got %s", fallbackRes.Data.Status)
	}
}

