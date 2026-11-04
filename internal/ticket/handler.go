package ticket

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/response"
)

// Handler processes HTTP requests for ticket resources.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all ticket endpoints onto the provided RouterGroup.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	tickets := rg.Group("/tickets")
	{
		tickets.POST("", h.CreateTicket)
		tickets.GET("", h.ListTickets)
		tickets.GET("/:id", h.GetTicketByID)
		tickets.PATCH("/:id", h.UpdateTicket)
		tickets.DELETE("/:id", h.DeleteTicket)
	}
}

// CreateTicket handles POST /api/v1/tickets
func (h *Handler) CreateTicket(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	ticket, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to create ticket")
		return
	}

	response.Created(c, ticket)
}

// ListTickets handles GET /api/v1/tickets?status=...&priority=...&limit=...&offset=...
func (h *Handler) ListTickets(c *gin.Context) {
	var filter ListTicketsFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.ValidationError(c, err)
		return
	}

	tickets, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to retrieve tickets")
		return
	}

	response.Paginated(c, tickets, total, filter.Limit, filter.Offset)
}

// GetTicketByID handles GET /api/v1/tickets/:id
func (h *Handler) GetTicketByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Ticket ID must be a positive integer")
		return
	}

	ticket, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to retrieve ticket")
		return
	}

	response.OK(c, ticket)
}

// UpdateTicket handles PATCH /api/v1/tickets/:id
func (h *Handler) UpdateTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Ticket ID must be a positive integer")
		return
	}

	var req UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	ticket, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to update ticket")
		return
	}

	response.OK(c, ticket)
}

// DeleteTicket handles DELETE /api/v1/tickets/:id
func (h *Handler) DeleteTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Ticket ID must be a positive integer")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to delete ticket")
		return
	}

	response.OK(c, gin.H{"deleted": true, "id": id})
}
