package label

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"ticketing-api/internal/project"
	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"

	"github.com/gin-gonic/gin"
)

type ProjectResolver interface {
	GetProjectByID(ctx context.Context, id int) (*project.Project, error)
}

type TicketResolver interface {
	GetByIDOrKey(ctx context.Context, idOrKey string) (*ticket.Ticket, error)
}

type Handler struct {
	service         *Service
	projectResolver ProjectResolver
	ticketResolver  TicketResolver
}

func NewHandler(service *Service, projectResolver ProjectResolver, ticketResolver TicketResolver) *Handler {
	return &Handler{
		service:         service,
		projectResolver: projectResolver,
		ticketResolver:  ticketResolver,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Project labels
	projects := rg.Group("/projects/:id/labels")
	{
		projects.GET("", h.ListProjectLabels)
		projects.POST("", authMiddleware, h.CreateProjectLabel)
	}

	// Ticket labels
	tickets := rg.Group("/tickets/:id_or_key/labels")
	{
		tickets.GET("", h.ListTicketLabels)
		tickets.POST("", authMiddleware, h.AddTicketLabel)
		tickets.DELETE("/:label_id", authMiddleware, h.RemoveTicketLabel)
	}
}

func (h *Handler) CreateProjectLabel(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	if h.projectResolver != nil {
		if _, err := h.projectResolver.GetProjectByID(c.Request.Context(), projectID); err != nil {
			if errors.Is(err, project.ErrProjectNotFound) {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
				return
			}
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to resolve project")
			return
		}
	}

	var req CreateLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	created, err := h.service.CreateProjectLabel(c.Request.Context(), projectID, req.Name, req.ColorHex)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "Label with this name already exists in this project")
			return
		}
		if errors.Is(err, ErrInvalidColor) {
			response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid color_hex: must be valid hex code like #EF4444")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	response.Created(c, created)
}

func (h *Handler) ListProjectLabels(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	labels, err := h.service.ListProjectLabels(c.Request.Context(), projectID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	if labels == nil {
		labels = make([]Label, 0)
	}

	response.OK(c, labels)
}

func (h *Handler) AddTicketLabel(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	t, err := h.ticketResolver.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to resolve ticket")
		return
	}

	var req AddTicketLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	if err := h.service.AddTicketLabel(c.Request.Context(), t.ID, req.LabelID); err != nil {
		if errors.Is(err, ErrAlreadyApplied) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "Label is already attached to this ticket")
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Label not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	labels, _ := h.service.ListTicketLabels(c.Request.Context(), t.ID)
	response.Created(c, labels)
}

func (h *Handler) RemoveTicketLabel(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	t, err := h.ticketResolver.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to resolve ticket")
		return
	}

	labelID, err := strconv.Atoi(c.Param("label_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid label ID")
		return
	}

	if err := h.service.RemoveTicketLabel(c.Request.Context(), t.ID, labelID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Label attachment not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	response.OK(c, gin.H{"deleted": true, "label_id": labelID})
}

func (h *Handler) ListTicketLabels(c *gin.Context) {
	idOrKey := c.Param("id_or_key")
	t, err := h.ticketResolver.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to resolve ticket")
		return
	}

	labels, err := h.service.ListTicketLabels(c.Request.Context(), t.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	if labels == nil {
		labels = make([]Label, 0)
	}

	response.OK(c, labels)
}
