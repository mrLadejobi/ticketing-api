package project

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/auth"
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
		projects.PATCH("/:id", h.UpdateProject)
		projects.GET("/:id/board", h.GetProjectBoard)
		projects.GET("/:id/timeline", h.GetProjectTimeline)
		projects.GET("/:id/logs", h.ListProjectLogs)

		// Project Members
		projects.POST("/:id/members", h.AddMember)
		projects.GET("/:id/members", h.ListMembers)
		projects.DELETE("/:id/members/:user_id", h.RemoveMember)
	}
}

func (h *Handler) CreateProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	creatorID := c.GetInt("user_id")
	p, err := h.service.CreateProject(c.Request.Context(), creatorID, req)
	if err != nil {
		if errors.Is(err, ErrDuplicatePrefix) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "Project prefix already in use")
			return
		}
		response.InternalError(c, err, "Failed to create project")
		return
	}

	response.Created(c, p)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	actorID := c.GetInt("user_id")
	updated, err := h.service.UpdateProject(c.Request.Context(), id, actorID, req)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
			return
		}
		response.InternalError(c, err, "Failed to update project")
		return
	}

	response.OK(c, updated)
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
		response.InternalError(c, err, "Failed to list projects")
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
		response.InternalError(c, err, "Failed to get project")
		return
	}

	response.OK(c, p)
}

func (h *Handler) AddMember(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	actorID := c.GetInt("user_id")
	member, err := h.service.AddMember(c.Request.Context(), projectID, actorID, req)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
			return
		}
		if errors.Is(err, ErrMemberDuplicate) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "User is already a member of this project")
			return
		}
		response.InternalError(c, err, "Failed to add member to project")
		return
	}

	response.Created(c, member)
}

func (h *Handler) ListMembers(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	members, err := h.service.ListMembers(c.Request.Context(), projectID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
			return
		}
		response.InternalError(c, err, "Failed to list project members")
		return
	}

	response.OK(c, members)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	targetUserID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || targetUserID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid user ID")
		return
	}

	requesterID := c.GetInt("user_id")
	requesterRole := auth.Role(c.GetString("user_role"))

	if err := h.service.RemoveMember(c.Request.Context(), projectID, targetUserID, requesterID, requesterRole); err != nil {
		if errors.Is(err, ErrProjectNotFound) || errors.Is(err, ErrMemberNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		response.InternalError(c, err, "Failed to remove project member")
		return
	}

	response.OK(c, gin.H{"message": "Member removed from project"})
}

func (h *Handler) ListProjectLogs(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := h.service.ListLogs(c.Request.Context(), projectID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
			return
		}
		response.InternalError(c, err, "Failed to list project logs")
		return
	}

	response.Paginated(c, logs, total, limit, offset)
}

func (h *Handler) GetProjectTimeline(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	p, err := h.service.GetProjectByID(c.Request.Context(), projectID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
			return
		}
		response.InternalError(c, err, "Failed to get project")
		return
	}

	var tickets []ticket.Ticket
	if h.ticketLister != nil {
		projectTickets, err := h.ticketLister.ListByProject(c.Request.Context(), projectID)
		if err != nil {
			response.InternalError(c, err, "Failed to load project tickets")
			return
		}
		tickets = projectTickets
	}

	now := time.Now().UTC()
	items := make([]TimelineItem, 0, len(tickets))
	for _, t := range tickets {
		isDone := t.Status == ticket.StatusDone || t.Status == ticket.StatusCompleted || t.Status == ticket.StatusCanceled
		isOverdue := t.DueDate != nil && t.DueDate.Before(now) && !isDone
		items = append(items, TimelineItem{
			TicketID:        t.ID,
			Key:             t.Key,
			Title:           t.Title,
			TicketType:      string(t.TicketType),
			Status:          string(t.Status),
			Priority:        string(t.Priority),
			ProgressPercent: t.ProgressPercent,
			DueDate:         t.DueDate,
			IsOverdue:       isOverdue,
			AssigneeID:      t.AssigneeID,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].DueDate == nil && items[j].DueDate == nil {
			return items[i].TicketID < items[j].TicketID
		}
		if items[i].DueDate == nil {
			return false
		}
		if items[j].DueDate == nil {
			return true
		}
		return items[i].DueDate.Before(*items[j].DueDate)
	})

	response.OK(c, TimelineResponse{
		Project: *p,
		Items:   items,
	})
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
		response.InternalError(c, err, "Failed to get project")
		return
	}

	var tickets []ticket.Ticket
	if h.ticketLister != nil {
		projectTickets, err := h.ticketLister.ListByProject(c.Request.Context(), id)
		if err != nil {
			response.InternalError(c, err, "Failed to load project tickets")
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
