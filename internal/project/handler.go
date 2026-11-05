package project

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"
)

type TicketLister interface {
	ListByProject(ctx context.Context, projectID int) ([]ticket.Ticket, error)
}

type Handler struct {
	service      *Service
	ticketLister TicketLister
}

func NewHandler(service *Service, ticketLister TicketLister) *Handler {
	return &Handler{
		service:      service,
		ticketLister: ticketLister,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	projects := rg.Group("/projects")
	projects.Use(authMiddleware)
	{
		projects.POST("", h.CreateProject)
		projects.GET("", h.ListProjects)
		projects.GET("/:id", h.GetProjectByID)
		projects.GET("/:id/board", h.GetProjectBoard)
	}
}

func (h *Handler) CreateProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	p, err := h.service.CreateProject(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicatePrefix) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "Project prefix already in use")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to create project")
		return
	}

	response.Created(c, p)
}

func (h *Handler) ListProjects(c *gin.Context) {
	var teamIDPtr *int
	if teamIDStr := c.Query("team_id"); teamIDStr != "" {
		if id, err := strconv.Atoi(teamIDStr); err == nil {
			teamIDPtr = &id
		}
	}

	projects, err := h.service.ListProjects(c.Request.Context(), teamIDPtr)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to list projects")
		return
	}

	response.OK(c, projects)
}

func (h *Handler) GetProjectByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	p, err := h.service.GetProjectByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to get project")
		return
	}

	response.OK(c, p)
}

// GetProjectBoard returns tickets partitioned into Kanban columns
func (h *Handler) GetProjectBoard(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	p, err := h.service.GetProjectByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to get project")
		return
	}

	var tickets []ticket.Ticket
	if h.ticketLister != nil {
		projectTickets, err := h.ticketLister.ListByProject(c.Request.Context(), id)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to load project tickets")
			return
		}
		tickets = projectTickets
	}

	// Group tickets into columns matching ticket.AllStatuses
	columnMap := make(map[ticket.TicketStatus][]ticket.Ticket)
	for _, status := range ticket.AllStatuses {
		columnMap[status] = make([]ticket.Ticket, 0)
	}

	for _, t := range tickets {
		columnMap[t.Status] = append(columnMap[t.Status], t)
	}

	columns := make([]BoardColumn, 0, len(ticket.AllStatuses))
	for _, status := range ticket.AllStatuses {
		ticketsInCol := columnMap[status]
		columns = append(columns, BoardColumn{
			Status:  string(status),
			Count:   len(ticketsInCol),
			Tickets: ticketsInCol,
		})
	}

	response.OK(c, BoardResponse{
		Project: *p,
		Columns: columns,
	})
}

