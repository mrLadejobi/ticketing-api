package cycle

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

type Handler struct {
	service         *Service
	projectResolver ProjectResolver
}

func NewHandler(service *Service, projectResolver ProjectResolver) *Handler {
	return &Handler{
		service:         service,
		projectResolver: projectResolver,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Nested under /projects/:id
	projects := rg.Group("/projects/:id/cycles")
	{
		projects.GET("", h.ListProjectCycles)
		projects.POST("", authMiddleware, h.CreateProjectCycle)
	}

	// Direct cycle operations
	cycles := rg.Group("/cycles/:id")
	{
		cycles.GET("", h.GetCycle)
		cycles.POST("/tickets", authMiddleware, h.AssignTicketsToCycle)
		cycles.GET("/analytics", h.GetCycleAnalytics)
		cycles.GET("/tickets", h.ListCycleTickets)
	}
}

func (h *Handler) CreateProjectCycle(c *gin.Context) {
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

	var req CreateCycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	created, err := h.service.Create(c.Request.Context(), projectID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidDates) {
			response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "start_date must be before end_date")
			return
		}
		if errors.Is(err, ErrAlreadyExists) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "Cycle with this name already exists in project")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	response.Created(c, created)
}

func (h *Handler) ListProjectCycles(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	cycles, err := h.service.ListByProjectID(c.Request.Context(), projectID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	if cycles == nil {
		cycles = make([]Cycle, 0)
	}

	response.OK(c, cycles)
}

func (h *Handler) GetCycle(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid cycle ID")
		return
	}

	cycle, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Cycle not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	response.OK(c, cycle)
}

func (h *Handler) AssignTicketsToCycle(c *gin.Context) {
	cycleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid cycle ID")
		return
	}

	var req AssignCycleTicketsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	if err := h.service.AssignTickets(c.Request.Context(), cycleID, req.TicketIDs); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Cycle not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	tickets, _ := h.service.ListCycleTickets(c.Request.Context(), cycleID)
	response.OK(c, gin.H{
		"assigned_count": len(req.TicketIDs),
		"cycle_id":       cycleID,
		"tickets":        tickets,
	})
}

func (h *Handler) GetCycleAnalytics(c *gin.Context) {
	cycleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid cycle ID")
		return
	}

	analytics, err := h.service.GetAnalytics(c.Request.Context(), cycleID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Cycle not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	response.OK(c, analytics)
}

func (h *Handler) ListCycleTickets(c *gin.Context) {
	cycleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid cycle ID")
		return
	}

	tickets, err := h.service.ListCycleTickets(c.Request.Context(), cycleID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Cycle not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, err.Error())
		return
	}

	if tickets == nil {
		tickets = make([]ticket.Ticket, 0)
	}

	response.OK(c, tickets)
}
