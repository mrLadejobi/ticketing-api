package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"

	"github.com/gin-gonic/gin"
)

var ticketKeyRegex = regexp.MustCompile(`\b([A-Z]{2,10}-\d+)\b`)

type TicketService interface {
	GetByIDOrKey(c context.Context, idOrKey string) (*ticket.Ticket, error)
	Update(c context.Context, id int, req ticket.UpdateTicketRequest) (*ticket.Ticket, error)
}

type CommentService interface {
	AddSystemComment(c context.Context, ticketID int, body string) error
}

// DeliveryTracker ensures webhook deliveries are processed idempotently.
type DeliveryTracker interface {
	RecordDelivery(ctx context.Context, deliveryID, eventType string) (bool, error)
}

type MemoryDeliveryTracker struct {
	mu         sync.Mutex
	deliveries map[string]bool
}

func NewMemoryDeliveryTracker() *MemoryDeliveryTracker {
	return &MemoryDeliveryTracker{deliveries: make(map[string]bool)}
}

func (m *MemoryDeliveryTracker) RecordDelivery(ctx context.Context, deliveryID, eventType string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deliveries[deliveryID] {
		return false, nil
	}
	m.deliveries[deliveryID] = true
	return true, nil
}

type PostgresDeliveryTracker struct {
	db *sql.DB
}

func NewPostgresDeliveryTracker(db *sql.DB) *PostgresDeliveryTracker {
	return &PostgresDeliveryTracker{db: db}
}

func (p *PostgresDeliveryTracker) RecordDelivery(ctx context.Context, deliveryID, eventType string) (bool, error) {
	query := `
		INSERT INTO webhook_deliveries (delivery_id, event_type, processed_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (delivery_id) DO NOTHING
	`
	res, err := p.db.ExecContext(ctx, query, deliveryID, eventType, time.Now().UTC())
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

type GitPayload struct {
	Action      string `json:"action"` // opened, closed, synchronize, reopened
	PullRequest *struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Merged  bool   `json:"merged"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		URL     string `json:"url"`
	} `json:"commits"`
}

type Handler struct {
	webhookSecret  string
	ticketService  TicketService
	commentService CommentService
	tracker        DeliveryTracker
}

func NewHandler(secret string, ticketService TicketService, commentService CommentService, tracker ...DeliveryTracker) *Handler {
	var dt DeliveryTracker
	if len(tracker) > 0 {
		dt = tracker[0]
	}
	return &Handler{
		webhookSecret:  secret,
		ticketService:  ticketService,
		commentService: commentService,
		tracker:        dt,
	}
}

func (h *Handler) SetDeliveryTracker(tracker DeliveryTracker) {
	h.tracker = tracker
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/webhooks/git", h.HandleGitWebhook)
}

func (h *Handler) HandleGitWebhook(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Failed to read webhook payload")
		return
	}

	// 1. Signature Verification (HMAC-SHA256) if secret is configured
	if h.webhookSecret != "" {
		sigHeader := c.GetHeader("X-Hub-Signature-256")
		if sigHeader == "" {
			sigHeader = c.GetHeader("X-Git-Signature")
		}
		if sigHeader == "" || !verifySignature(h.webhookSecret, bodyBytes, sigHeader) {
			response.Error(c, http.StatusUnauthorized, "INVALID_SIGNATURE", "Webhook signature verification failed")
			return
		}
	}

	// 2. Delivery Idempotency Check (Deduplication)
	deliveryID := c.GetHeader("X-GitHub-Delivery")
	if deliveryID == "" {
		deliveryID = c.GetHeader("X-Gitlab-Event-UUID")
	}
	if deliveryID == "" {
		hash := sha256.Sum256(bodyBytes)
		deliveryID = "sha256_" + hex.EncodeToString(hash[:])
	}

	eventType := c.GetHeader("X-GitHub-Event")
	if eventType == "" {
		eventType = c.GetHeader("X-Gitlab-Event")
	}

	if h.tracker != nil {
		isNew, err := h.tracker.RecordDelivery(c.Request.Context(), deliveryID, eventType)
		if err == nil && !isNew {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"status":      "duplicate_ignored",
					"delivery_id": deliveryID,
				},
			})
			return
		}
	}

	// 3. Parse Git Payload
	var payload GitPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid JSON payload")
		return
	}

	// 4. Extract Ticket Keys from PR & Commits
	ticketKeys := make(map[string]bool)

	if payload.PullRequest != nil {
		for _, key := range ticketKeyRegex.FindAllString(payload.PullRequest.Title, -1) {
			ticketKeys[strings.ToUpper(key)] = true
		}
		for _, key := range ticketKeyRegex.FindAllString(payload.PullRequest.Body, -1) {
			ticketKeys[strings.ToUpper(key)] = true
		}
		for _, key := range ticketKeyRegex.FindAllString(payload.PullRequest.Head.Ref, -1) {
			ticketKeys[strings.ToUpper(key)] = true
		}
	}

	for _, commit := range payload.Commits {
		for _, key := range ticketKeyRegex.FindAllString(commit.Message, -1) {
			ticketKeys[strings.ToUpper(key)] = true
		}
	}

	var transitioned []string
	ctx := c.Request.Context()

	// 4. Auto-transition Tickets
	for key := range ticketKeys {
		t, err := h.ticketService.GetByIDOrKey(ctx, key)
		if err != nil {
			continue
		}

		if payload.PullRequest != nil {
			pr := payload.PullRequest
			if pr.Merged {
				// PR merged -> transition to DONE
				doneStatus := ticket.StatusDone
				_, _ = h.ticketService.Update(ctx, t.ID, ticket.UpdateTicketRequest{
					Status: &doneStatus,
				})
				transitioned = append(transitioned, fmt.Sprintf("%s -> DONE (PR #%d merged)", key, pr.Number))

				if h.commentService != nil {
					_ = h.commentService.AddSystemComment(ctx, t.ID, fmt.Sprintf("🎉 **Git Webhook**: Pull Request [#%d](%s) was merged: *%s*. Ticket transitioned to `DONE`.", pr.Number, pr.HTMLURL, pr.Title))
				}
			} else if payload.Action == "opened" || payload.Action == "synchronize" || payload.Action == "reopened" {
				// PR opened / updated -> transition to IN_REVIEW
				if t.Status != ticket.StatusDone && t.Status != ticket.StatusInReview {
					reviewStatus := ticket.StatusInReview
					_, _ = h.ticketService.Update(ctx, t.ID, ticket.UpdateTicketRequest{
						Status: &reviewStatus,
					})
					transitioned = append(transitioned, fmt.Sprintf("%s -> IN_REVIEW (PR #%d opened)", key, pr.Number))

					if h.commentService != nil {
						_ = h.commentService.AddSystemComment(ctx, t.ID, fmt.Sprintf("🔗 **Git Webhook**: Linked Pull Request [#%d](%s): *%s* (branch: `%s`). Ticket transitioned to `IN_REVIEW`.", pr.Number, pr.HTMLURL, pr.Title, pr.Head.Ref))
					}
				}
			}
		}
	}

	response.OK(c, gin.H{
		"processed_keys": len(ticketKeys),
		"transitioned":   transitioned,
	})
}

func verifySignature(secret string, payload []byte, sigHeader string) bool {
	sig := strings.TrimPrefix(sigHeader, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expectedMAC))
}
