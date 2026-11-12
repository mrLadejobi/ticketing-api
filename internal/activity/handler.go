package activity

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"

	"github.com/gin-gonic/gin"
)

type TicketResolver interface {
	GetByIDOrKey(ctx context.Context, idOrKey string) (*ticket.Ticket, error)
}

type Handler struct {
	service        *Service
	ticketResolver TicketResolver
}

func NewHandler(service *Service, ticketResolver TicketResolver) *Handler {
	return &Handler{
		service:        service,
		ticketResolver: ticketResolver,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware, adminMiddleware gin.HandlerFunc) {
	// Nested under /tickets/:id_or_key/activity
	rg.GET("/tickets/:id_or_key/activity", h.ListTicketActivities)

	// Admin-only system-wide security & mutation audit trail
	rg.GET("/audit-logs", authMiddleware, adminMiddleware, h.ListAuditLogs)
}

func (h *Handler) ListTicketActivities(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	t, err := h.ticketResolver.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to resolve ticket")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	activities, total, err := h.service.ListTicketActivities(c.Request.Context(), t.ID, limit, offset)
	if err != nil {
		response.InternalError(c, err, err.Error())
		return
	}

	if activities == nil {
		activities = make([]TicketActivity, 0)
	}

	response.Paginated(c, activities, total, limit, offset)
}

func (h *Handler) ListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	logs, total, err := h.service.ListAuditLogs(c.Request.Context(), limit, offset)
	if err != nil {
		response.InternalError(c, err, err.Error())
		return
	}

	if logs == nil {
		logs = make([]AuditLog, 0)
	}

	response.Paginated(c, logs, total, limit, offset)
}
