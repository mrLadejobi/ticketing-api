package export

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"ticketing-api/internal/auth"
	"ticketing-api/internal/project"
	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"

	"github.com/gin-gonic/gin"
)

type TicketReader interface {
	List(ctx context.Context, filter ticket.ListTicketsFilter) ([]ticket.Ticket, int, error)
}

type UserReader interface {
	GetUserByID(ctx context.Context, id int) (*auth.User, error)
}

type ProjectLister interface {
	ListProjects(ctx context.Context, teamID *int) ([]project.Project, error)
}

type Handler struct {
	tickets  TicketReader
	users    UserReader
	projects ProjectLister
}

func NewHandler(tickets TicketReader, users UserReader, projects ProjectLister) *Handler {
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
		users.GET("/me/brag-export", h.ExportMyBrag)
		users.GET("/:id/brag-export", h.ExportUserBrag)
	}
}

func (h *Handler) ExportMyBrag(c *gin.Context) {
	userID := c.GetInt("user_id")
	h.exportForUser(c, userID)
}

func (h *Handler) ExportUserBrag(c *gin.Context) {
	targetID, err := strconv.Atoi(c.Param("id"))
	if err != nil || targetID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid user ID")
		return
	}

	requesterID := c.GetInt("user_id")
	requesterRole := auth.Role(c.GetString("user_role"))
	if requesterID != targetID && requesterRole != auth.RoleAdmin {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "Only the user themselves or an ADMIN can export this brag report")
		return
	}

	h.exportForUser(c, targetID)
}

func (h *Handler) exportForUser(c *gin.Context, userID int) {
	ctx := c.Request.Context()

	startDate, endDate, err := parseDateWindow(c.Query("start_date"), c.Query("end_date"), c.Query("period"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, err.Error())
		return
	}

	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "pdf")))
	if format != "pdf" && format != "markdown" && format != "json" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "format must be one of: pdf, markdown, json")
		return
	}

	orgName := strings.TrimSpace(c.DefaultQuery("org_name", "Engineering Platform"))

	u, err := h.users.GetUserByID(ctx, userID)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "User not found")
		return
	}

	assigned, _, err := h.tickets.List(ctx, ticket.ListTicketsFilter{
		AssigneeID: &userID,
		Limit:      1000,
	})
	if err != nil {
		response.InternalError(c, err, "Failed to load user tickets for brag export")
		return
	}

	projByID := make(map[int]project.Project)
	if h.projects != nil {
		if projs, err := h.projects.ListProjects(ctx, nil); err == nil {
			for _, p := range projs {
				projByID[p.ID] = p
			}
		}
	}

	report := buildBragReport(u, orgName, startDate, endDate, assigned, projByID)

	switch format {
	case "json":
		response.OK(c, report)
	case "markdown":
		md := RenderBragMarkdown(report)
		filename := fmt.Sprintf("brag-report-%s-%s.md", startDate.Format("20060102"), endDate.Format("20060102"))
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(md))
	default:
		pdfBytes, err := RenderBragPDF(report)
		if err != nil {
			response.InternalError(c, err, "Failed to generate brag PDF")
			return
		}
		filename := fmt.Sprintf("brag-report-%s-%s.pdf", startDate.Format("20060102"), endDate.Format("20060102"))
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		c.Data(http.StatusOK, "application/pdf", pdfBytes)
	}
}

func parseDateWindow(startStr, endStr, period string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

	days := 30
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "7d":
		days = 7
	case "30d", "":
		days = 30
	case "90d":
		days = 90
	case "180d":
		days = 180
	case "365d":
		days = 365
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported period %q (use 7d, 30d, 90d, 180d, 365d, or explicit start_date/end_date)", period)
	}
	startDate := endDate.AddDate(0, 0, -days).Truncate(24 * time.Hour)

	if startStr != "" {
		parsed, err := parseFlexibleDate(startStr, false)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start_date: %w", err)
		}
		startDate = parsed
	}
	if endStr != "" {
		parsed, err := parseFlexibleDate(endStr, true)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end_date: %w", err)
		}
		endDate = parsed
	}

	if startDate.After(endDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date must be before or equal to end_date")
	}

	return startDate, endDate, nil
}

func parseFlexibleDate(raw string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		if endOfDay {
			return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC), nil
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("expected YYYY-MM-DD or RFC3339")
}

func buildBragReport(
	u *auth.User,
	orgName string,
	startDate, endDate time.Time,
	assigned []ticket.Ticket,
	projByID map[int]project.Project,
) *BragReport {
	report := &BragReport{
		OrgName: orgName,
		User: auth.UserSummary{
			ID:       u.ID,
			Email:    u.Email,
			FullName: u.FullName,
			Role:     u.Role,
		},
		StartDate:     startDate,
		EndDate:       endDate,
		GeneratedAt:   time.Now().UTC(),
		TypeBreakdown: make(map[string]int),
		CompletedWork: make([]BragTaskItem, 0),
		ActiveWork:    make([]BragTaskItem, 0),
	}

	projStats := make(map[int]*ProjectStat)

	for _, t := range assigned {
		if t.UpdatedAt.Before(startDate) || t.UpdatedAt.After(endDate) {
			continue
		}

		projName := fmt.Sprintf("Project #%d", t.ProjectID)
		prefix := "PRJ"
		if p, ok := projByID[t.ProjectID]; ok {
			projName = p.Name
			prefix = p.Prefix
		}

		tType := string(t.TicketType)
		if tType == "" {
			tType = string(ticket.TypeTask)
		}
		report.TypeBreakdown[tType]++
		report.Summary.TotalTasks++

		ps, exists := projStats[t.ProjectID]
		if !exists {
			ps = &ProjectStat{
				ProjectID:   t.ProjectID,
				ProjectName: projName,
				Prefix:      prefix,
			}
			projStats[t.ProjectID] = ps
		}
		ps.Total++

		item := BragTaskItem{
			ID:              t.ID,
			Key:             t.Key,
			ProjectName:     projName,
			TicketType:      tType,
			Title:           t.Title,
			Description:     t.Description,
			WorkSummary:     t.WorkSummary,
			Status:          string(t.Status),
			Priority:        string(t.Priority),
			ProgressPercent: t.ProgressPercent,
			DueDate:         t.DueDate,
			UpdatedAt:       t.UpdatedAt,
		}

		isCompleted := t.Status == ticket.StatusDone || t.Status == ticket.StatusCompleted
		isOnHold := t.Status == ticket.StatusOnHold || t.Status == ticket.StatusBlocked
		if isCompleted {
			report.Summary.CompletedTasks++
			ps.Completed++
			switch t.TicketType {
			case ticket.TypeFeature:
				report.Summary.FeaturesShipped++
			case ticket.TypeBug:
				report.Summary.BugsResolved++
			}
			report.CompletedWork = append(report.CompletedWork, item)
		} else {
			if isOnHold {
				report.Summary.OnHoldTasks++
			} else if t.Status != ticket.StatusCanceled {
				report.Summary.InProgressTasks++
			}
			report.ActiveWork = append(report.ActiveWork, item)
		}
	}

	if report.Summary.TotalTasks > 0 {
		report.Summary.CompletionRate = float64(report.Summary.CompletedTasks) / float64(report.Summary.TotalTasks) * 100.0
	}

	for _, ps := range projStats {
		if ps.Total > 0 {
			ps.CompletionRate = float64(ps.Completed) / float64(ps.Total) * 100.0
		}
		report.ProjectBreakdown = append(report.ProjectBreakdown, *ps)
	}
	sort.Slice(report.ProjectBreakdown, func(i, j int) bool {
		return report.ProjectBreakdown[i].Total > report.ProjectBreakdown[j].Total
	})

	if report.Summary.TotalTasks == 0 {
		report.KeyInsight = "No tasks were updated during this period. Adjust start_date/end_date or period to widen your review window."
	} else {
		report.KeyInsight = fmt.Sprintf(
			"Completed %d of %d tasks (%.1f%% delivery rate), shipping %d feature(s) and resolving %d bug(s) across %d project(s).",
			report.Summary.CompletedTasks,
			report.Summary.TotalTasks,
			report.Summary.CompletionRate,
			report.Summary.FeaturesShipped,
			report.Summary.BugsResolved,
			len(report.ProjectBreakdown),
		)
	}

	return report
}
