package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/project"
	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"
)

type TicketReader interface {
	List(ctx context.Context, filter ticket.ListTicketsFilter) ([]ticket.Ticket, int, error)
	ListByProject(ctx context.Context, projectID int) ([]ticket.Ticket, error)
}

type UserReader interface {
	GetUserByID(ctx context.Context, id int) (*auth.User, error)
}

type ProjectReader interface {
	GetProjectByID(ctx context.Context, id int) (*project.Project, error)
}

type TaskWorkItem struct {
	ID              int        `json:"id"`
	Key             string     `json:"key"`
	TicketType      string     `json:"ticket_type"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	WorkSummary     string     `json:"work_summary"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	ProgressPercent *int       `json:"progress_percent,omitempty"`
	DueDate         *time.Time `json:"due_date,omitempty"`
	IsOverdue       bool       `json:"is_overdue"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserMetricsResponse struct {
	UserID                int               `json:"user_id"`
	User                  *auth.UserSummary `json:"user,omitempty"`
	TotalAssigned         int               `json:"total_assigned"`
	TasksCompleted        int               `json:"tasks_completed"`
	TasksInProgress       int               `json:"tasks_in_progress"`
	TasksOnHold           int               `json:"tasks_on_hold"`
	CompletionRatePercent float64           `json:"completion_rate_percent"`
	TasksNeedingAttention []TaskWorkItem    `json:"tasks_needing_attention"`
	WorkHistory           []TaskWorkItem    `json:"work_history"`
}

type MemberTaskStat struct {
	UserID          int    `json:"user_id"`
	FullName        string `json:"full_name"`
	Email           string `json:"email"`
	AssignedCount   int    `json:"assigned_count"`
	CompletedCount  int    `json:"completed_count"`
	InProgressCount int    `json:"in_progress_count"`
}

type ProjectMetricsResponse struct {
	ProjectID             int              `json:"project_id"`
	ProjectName           string           `json:"project_name"`
	Prefix                string           `json:"prefix"`
	TotalTasks            int              `json:"total_tasks"`
	CompletedTasks        int              `json:"completed_tasks"`
	CompletionPercentage  float64          `json:"completion_percentage"`
	OverdueTasks          int              `json:"overdue_tasks"`
	TasksNeedingAttention []TaskWorkItem   `json:"tasks_needing_attention"`
	BreakdownByStatus     map[string]int   `json:"breakdown_by_status"`
	BreakdownByType       map[string]int   `json:"breakdown_by_type"`
	MemberStats           []MemberTaskStat `json:"member_stats"`
}

type Handler struct {
	tickets  TicketReader
	users    UserReader
	projects ProjectReader
}

func NewHandler(tickets TicketReader, users UserReader, projects ProjectReader) *Handler {
	return &Handler{
		tickets:  tickets,
		users:    users,
		projects: projects,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	users := rg.Group("/users")
	users.Use(authMiddleware)
	{
		users.GET("/me/metrics", h.GetMyMetrics)
		users.GET("/:id/metrics", h.GetUserMetrics)
	}

	projects := rg.Group("/projects")
	projects.Use(authMiddleware)
	{
		projects.GET("/:id/metrics", h.GetProjectMetrics)
	}
}

func isCompletedStatus(s ticket.TicketStatus) bool {
	return s == ticket.StatusDone || s == ticket.StatusCompleted
}

func isInProgressStatus(s ticket.TicketStatus) bool {
	return s == ticket.StatusStarted || s == ticket.StatusInProgress || s == ticket.StatusInReview || s == ticket.StatusQATesting
}

func isOnHoldStatus(s ticket.TicketStatus) bool {
	return s == ticket.StatusOnHold || s == ticket.StatusBlocked
}

func toWorkItem(t ticket.Ticket, now time.Time) (TaskWorkItem, bool) {
	isDone := isCompletedStatus(t.Status) || t.Status == ticket.StatusCanceled
	isOverdue := t.DueDate != nil && t.DueDate.Before(now) && !isDone
	needsAttention := !isDone && (isOverdue || t.Status == ticket.StatusBlocked || t.Priority == ticket.PriorityUrgent)

	return TaskWorkItem{
		ID:              t.ID,
		Key:             t.Key,
		TicketType:      string(t.TicketType),
		Title:           t.Title,
		Description:     t.Description,
		WorkSummary:     t.WorkSummary,
		Status:          string(t.Status),
		Priority:        string(t.Priority),
		ProgressPercent: t.ProgressPercent,
		DueDate:         t.DueDate,
		IsOverdue:       isOverdue,
		UpdatedAt:       t.UpdatedAt,
	}, needsAttention
}

func (h *Handler) GetMyMetrics(c *gin.Context) {
	userID := c.GetInt("user_id")
	h.computeUserMetrics(c, userID)
}

func (h *Handler) GetUserMetrics(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid user ID")
		return
	}
	h.computeUserMetrics(c, userID)
}

func (h *Handler) computeUserMetrics(c *gin.Context, userID int) {
	ctx := c.Request.Context()

	var userSummary *auth.UserSummary
	if h.users != nil {
		u, err := h.users.GetUserByID(ctx, userID)
		if err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "User not found")
			return
		}
		userSummary = &auth.UserSummary{
			ID:       u.ID,
			Email:    u.Email,
			FullName: u.FullName,
			Role:     u.Role,
		}
	}

	assignedTickets, _, err := h.tickets.List(ctx, ticket.ListTicketsFilter{
		AssigneeID: &userID,
		Limit:      1000,
	})
	if err != nil {
		response.InternalError(c, err, "Failed to load user metrics")
		return
	}

	now := time.Now().UTC()
	res := UserMetricsResponse{
		UserID:                userID,
		User:                  userSummary,
		TotalAssigned:         len(assignedTickets),
		TasksNeedingAttention: make([]TaskWorkItem, 0),
		WorkHistory:           make([]TaskWorkItem, 0, len(assignedTickets)),
	}

	for _, t := range assignedTickets {
		if isCompletedStatus(t.Status) {
			res.TasksCompleted++
		} else if isInProgressStatus(t.Status) {
			res.TasksInProgress++
		} else if isOnHoldStatus(t.Status) {
			res.TasksOnHold++
		}

		item, needsAttention := toWorkItem(t, now)
		res.WorkHistory = append(res.WorkHistory, item)
		if needsAttention {
			res.TasksNeedingAttention = append(res.TasksNeedingAttention, item)
		}
	}

	if res.TotalAssigned > 0 {
		res.CompletionRatePercent = float64(res.TasksCompleted) / float64(res.TotalAssigned) * 100.0
	}

	response.OK(c, res)
}

func (h *Handler) GetProjectMetrics(c *gin.Context) {
	ctx := c.Request.Context()
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid project ID")
		return
	}

	proj, err := h.projects.GetProjectByID(ctx, projectID)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Project not found")
		return
	}

	projectTickets, err := h.tickets.ListByProject(ctx, projectID)
	if err != nil {
		response.InternalError(c, err, "Failed to load project metrics")
		return
	}

	now := time.Now().UTC()
	res := ProjectMetricsResponse{
		ProjectID:             proj.ID,
		ProjectName:           proj.Name,
		Prefix:                proj.Prefix,
		TotalTasks:            len(projectTickets),
		TasksNeedingAttention: make([]TaskWorkItem, 0),
		BreakdownByStatus:     make(map[string]int),
		BreakdownByType:       make(map[string]int),
		MemberStats:           make([]MemberTaskStat, 0),
	}

	memberMap := make(map[int]*MemberTaskStat)

	for _, t := range projectTickets {
		res.BreakdownByStatus[string(t.Status)]++
		res.BreakdownByType[string(t.TicketType)]++

		if isCompletedStatus(t.Status) {
			res.CompletedTasks++
		}

		item, needsAttention := toWorkItem(t, now)
		if item.IsOverdue {
			res.OverdueTasks++
		}
		if needsAttention {
			res.TasksNeedingAttention = append(res.TasksNeedingAttention, item)
		}

		if t.AssigneeID != nil {
			stat, exists := memberMap[*t.AssigneeID]
			if !exists {
				stat = &MemberTaskStat{UserID: *t.AssigneeID}
				if t.Assignee != nil {
					stat.FullName = t.Assignee.FullName
					stat.Email = t.Assignee.Email
				}
				memberMap[*t.AssigneeID] = stat
			}
			stat.AssignedCount++
			if isCompletedStatus(t.Status) {
				stat.CompletedCount++
			} else if isInProgressStatus(t.Status) {
				stat.InProgressCount++
			}
		}
	}

	if res.TotalTasks > 0 {
		res.CompletionPercentage = float64(res.CompletedTasks) / float64(res.TotalTasks) * 100.0
	}

	for _, s := range memberMap {
		res.MemberStats = append(res.MemberStats, *s)
	}

	response.OK(c, res)
}

