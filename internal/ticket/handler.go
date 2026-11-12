package ticket

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"ticketing-api/internal/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all enterprise ticket endpoints onto the provided RouterGroup.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware, adminMiddleware gin.HandlerFunc) {
	tickets := rg.Group("/tickets")
	{
		// Public or Authenticated Read
		tickets.GET("", h.ListTickets)
		tickets.GET("/:id_or_key", h.GetTicketByIDOrKey)
		tickets.GET("/:id_or_key/subtasks", h.ListSubtasks)

		// Protected Operations (requires authenticated user session)
		tickets.PATCH("/bulk", authMiddleware, h.BulkUpdateTickets)
		tickets.POST("", authMiddleware, h.CreateTicket)
		tickets.PATCH("/:id_or_key", authMiddleware, h.UpdateTicket)
		tickets.DELETE("/:id_or_key", authMiddleware, h.DeleteTicket)

		// Admin-Only Restoration of Soft-Deleted Tickets
		tickets.POST("/:id_or_key/restore", authMiddleware, adminMiddleware, h.RestoreTicket)
	}
}

// CreateTicket handles POST /api/v1/tickets
func (h *Handler) CreateTicket(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	reporterID := userIDVal.(int)

	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	ticket, err := h.service.Create(c.Request.Context(), reporterID, req)
	if err != nil {
		response.InternalError(c, err, "Failed to create ticket: "+err.Error())
		return
	}

	response.Created(c, ticket)
}

// ListTickets handles GET /api/v1/tickets
func (h *Handler) ListTickets(c *gin.Context) {
	var filter ListTicketsFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.ValidationError(c, err)
		return
	}

	tickets, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err, "Failed to retrieve tickets")
		return
	}

	response.Paginated(c, tickets, total, filter.Limit, filter.Offset)
}

// GetTicketByIDOrKey handles GET /api/v1/tickets/:id_or_key
func (h *Handler) GetTicketByIDOrKey(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	if idOrKey == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Ticket ID or key required")
		return
	}

	ticket, err := h.service.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to retrieve ticket")
		return
	}

	response.OK(c, ticket)
}

// UpdateTicket handles PATCH /api/v1/tickets/:id_or_key with Optimistic Locking
func (h *Handler) UpdateTicket(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	existing, err := h.service.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to retrieve ticket")
		return
	}

	var req UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	// Check HTTP If-Match header for optimistic locking if provided
	if ifMatch := c.GetHeader("If-Match"); ifMatch != "" {
		if v, err := strconv.Atoi(ifMatch); err == nil && req.Version == nil {
			req.Version = &v
		}
	}

	ctx := c.Request.Context()
	if uid, exists := c.Get("user_id"); exists {
		ctx = context.WithValue(ctx, "user_id", uid)
	}

	ticket, err := h.service.Update(ctx, existing.ID, req)
	if err != nil {
		if errors.Is(err, ErrOptimisticLockConflict) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "Optimistic lock failure: ticket was modified by another user. Please reload.")
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to update ticket: "+err.Error())
		return
	}

	response.OK(c, ticket)
}

// DeleteTicket handles DELETE /api/v1/tickets/:id_or_key (Soft Delete)
func (h *Handler) DeleteTicket(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	existing, err := h.service.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to find ticket")
		return
	}

	if err := h.service.SoftDelete(c.Request.Context(), existing.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found or already deleted")
			return
		}
		response.InternalError(c, err, "Failed to delete ticket")
		return
	}

	response.OK(c, gin.H{"deleted": true, "key": existing.Key, "id": existing.ID})
}

// RestoreTicket handles POST /api/v1/tickets/:id_or_key/restore (Admin Only)
func (h *Handler) RestoreTicket(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	existing, err := h.service.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to find ticket")
		return
	}

	restored, err := h.service.Restore(c.Request.Context(), existing.ID)
	if err != nil {
		response.InternalError(c, err, "Failed to restore ticket")
		return
	}

	response.OK(c, restored)
}

// ListSubtasks handles GET /api/v1/tickets/:id_or_key/subtasks
func (h *Handler) ListSubtasks(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	parent, err := h.service.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Parent ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to resolve parent ticket")
		return
	}

	subtasks, err := h.service.ListSubtasks(c.Request.Context(), parent.ID)
	if err != nil {
		response.InternalError(c, err, "Failed to retrieve subtasks")
		return
	}

	if typeFilter := strings.ToUpper(strings.TrimSpace(c.Query("type"))); typeFilter != "" {
		filtered := make([]Ticket, 0, len(subtasks))
		for _, st := range subtasks {
			if string(st.TicketType) == typeFilter {
				filtered = append(filtered, st)
			}
		}
		subtasks = filtered
	}

	response.OK(c, subtasks)
}

// BulkUpdateTickets handles PATCH /api/v1/tickets/bulk
func (h *Handler) BulkUpdateTickets(c *gin.Context) {
	var req BulkUpdateTicketsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	ctx := c.Request.Context()
	if uid, exists := c.Get("user_id"); exists {
		ctx = context.WithValue(ctx, "user_id", uid)
	}

	updated, err := h.service.BulkUpdate(ctx, req)
	if err != nil {
		response.InternalError(c, err, err.Error())
		return
	}

	response.OK(c, gin.H{
		"updated_count": len(updated),
		"tickets":       updated,
	})
}
