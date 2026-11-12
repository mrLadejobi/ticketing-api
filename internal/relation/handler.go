package relation

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

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Nested under /tickets/:id_or_key
	tickets := rg.Group("/tickets/:id_or_key/relations")
	{
		tickets.GET("", h.ListRelations)
		tickets.POST("", authMiddleware, h.CreateRelation)
		tickets.DELETE("/:relation_id", authMiddleware, h.DeleteRelation)
	}

	// Flat route for deleting relation by ID directly
	rg.DELETE("/relations/:relation_id", authMiddleware, h.DeleteRelation)
}

func (h *Handler) CreateRelation(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	sourceTicket, err := h.ticketResolver.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Source ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to resolve source ticket")
		return
	}

	var req CreateRelationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	targetTicket, err := h.ticketResolver.GetByIDOrKey(c.Request.Context(), req.TargetTicketIDOrKey)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Target ticket not found")
			return
		}
		response.InternalError(c, err, "Failed to resolve target ticket")
		return
	}

	if sourceTicket.ID == targetTicket.ID {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Cannot create a relation between a ticket and itself")
		return
	}

	created, err := h.service.Create(c.Request.Context(), sourceTicket.ID, targetTicket.ID, req.RelationType)
	if err != nil {
		if errors.Is(err, ErrRelationExists) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "A relation between these tickets already exists")
			return
		}
		if errors.Is(err, ErrSelfRelation) {
			response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Cannot create a relation between a ticket and itself")
			return
		}
		response.InternalError(c, err, err.Error())
		return
	}

	response.Created(c, created)
}

func (h *Handler) ListRelations(c *gin.Context) {
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

	relations, err := h.service.ListByTicketID(c.Request.Context(), t.ID)
	if err != nil {
		response.InternalError(c, err, err.Error())
		return
	}

	if relations == nil {
		relations = make([]TicketRelation, 0)
	}

	response.OK(c, relations)
}

func (h *Handler) DeleteRelation(c *gin.Context) {
	relID, err := strconv.Atoi(c.Param("relation_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid relation ID")
		return
	}

	if err := h.service.Delete(c.Request.Context(), relID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket relation not found")
			return
		}
		response.InternalError(c, err, err.Error())
		return
	}

	response.OK(c, gin.H{"deleted": true, "relation_id": relID})
}
