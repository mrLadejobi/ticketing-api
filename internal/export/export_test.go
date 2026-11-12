package export_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/export"
	"ticketing-api/internal/project"
	"ticketing-api/internal/ticket"
)

func setupExportEnv(t *testing.T) (*auth.User, *auth.User, *export.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	userRepo := auth.NewMemoryRepository()
	u1, _ := userRepo.CreateUser(context.Background(), &auth.User{
		Email:    "saanumi@company.com",
		FullName: "Oluwasaanumi Ladejobi",
		Role:     auth.RoleMember,
	})
	admin, _ := userRepo.CreateUser(context.Background(), &auth.User{
		Email:    "admin@company.com",
		FullName: "Engineering Lead",
		Role:     auth.RoleAdmin,
	})

	projRepo := project.NewMemoryRepository(userRepo)
	projSvc := project.NewService(projRepo)
	proj, _ := projSvc.CreateProject(context.Background(), u1.ID, project.CreateProjectRequest{
		TeamID: 1,
		Name:   "Core Ticketing Engine",
		Prefix: "ENG",
	})

	ticketRepo := ticket.NewMemoryRepository(userRepo)
	ticketSvc := ticket.NewService(ticketRepo, projSvc)

	prog100 := 100
	prog65 := 65
	_, _ = ticketSvc.Create(context.Background(), u1.ID, ticket.CreateTicketRequest{
		ProjectID:       proj.ID,
		TicketType:      ticket.TypeFeature,
		Title:           "Build Brag Document PDF Exporter",
		Description:     "Generate executive performance PDFs for review cycles",
		WorkSummary:     "Designed pure-Go fpdf layout with KPI cards, stacked charts, and markdown fallback.",
		Status:          ticket.StatusCompleted,
		Priority:        ticket.PriorityHigh,
		ProgressPercent: &prog100,
		AssigneeID:      &u1.ID,
	})
	_, _ = ticketSvc.Create(context.Background(), u1.ID, ticket.CreateTicketRequest{
		ProjectID:       proj.ID,
		TicketType:      ticket.TypeBug,
		Title:           "Fix transaction read isolation",
		Description:     "Ensure GetByIDOrKey uses active sql.Tx executor",
		WorkSummary:     "Wired context tx executor across all ticket read queries.",
		Status:          ticket.StatusInProgress,
		Priority:        ticket.PriorityUrgent,
		ProgressPercent: &prog65,
		AssigneeID:      &u1.ID,
	})

	h := export.NewHandler(ticketRepo, userRepo, projSvc)
	return u1, admin, h
}

func makeRouter(userID int, role auth.Role, h *export.Handler) *gin.Engine {
	r := gin.New()
	dummyAuth := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", string(role))
		c.Next()
	}
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1, dummyAuth)
	return r
}

func TestBragExport_PDF_Markdown_JSON(t *testing.T) {
	u1, _, h := setupExportEnv(t)
	r := makeRouter(u1.ID, auth.RoleMember, h)

	// 1. Default PDF export
	reqPDF, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me/brag-export?period=30d&org_name=Viscount+Engineering", nil)
	wPDF := httptest.NewRecorder()
	r.ServeHTTP(wPDF, reqPDF)

	if wPDF.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PDF export, got %d: %s", wPDF.Code, wPDF.Body.String())
	}
	if ct := wPDF.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected application/pdf Content-Type, got %s", ct)
	}
	if !bytes.HasPrefix(wPDF.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("expected valid %%PDF- header bytes")
	}

	// 2. Markdown export
	reqMD, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me/brag-export?format=markdown", nil)
	wMD := httptest.NewRecorder()
	r.ServeHTTP(wMD, reqMD)

	if wMD.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for markdown export, got %d", wMD.Code)
	}
	mdBody := wMD.Body.String()
	if !strings.Contains(mdBody, "Oluwasaanumi Ladejobi") || !strings.Contains(mdBody, "Designed pure-Go fpdf layout") {
		t.Fatalf("markdown export missing expected content: %s", mdBody)
	}

	// 3. JSON preview
	reqJSON, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me/brag-export?format=json", nil)
	wJSON := httptest.NewRecorder()
	r.ServeHTTP(wJSON, reqJSON)

	if wJSON.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for json preview, got %d", wJSON.Code)
	}
	var res struct {
		Data export.BragReport `json:"data"`
	}
	if err := json.Unmarshal(wJSON.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json preview: %v", err)
	}
	if res.Data.Summary.TotalTasks != 2 || res.Data.Summary.CompletedTasks != 1 || res.Data.Summary.FeaturesShipped != 1 {
		t.Fatalf("unexpected brag summary: %+v", res.Data.Summary)
	}

	// 4. Date window in the past excludes today's tasks
	pastEnd := time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02")
	pastStart := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02")
	reqPast, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me/brag-export?format=json&start_date="+pastStart+"&end_date="+pastEnd, nil)
	wPast := httptest.NewRecorder()
	r.ServeHTTP(wPast, reqPast)
	var pastRes struct {
		Data export.BragReport `json:"data"`
	}
	_ = json.Unmarshal(wPast.Body.Bytes(), &pastRes)
	if pastRes.Data.Summary.TotalTasks != 0 {
		t.Fatalf("expected 0 tasks for past window, got %d", pastRes.Data.Summary.TotalTasks)
	}
}

func TestBragExport_Authorization(t *testing.T) {
	u1, admin, h := setupExportEnv(t)

	// Member 99 trying to export User 1's report -> 403 Forbidden
	otherRouter := makeRouter(99, auth.RoleMember, h)
	reqForbid, _ := http.NewRequest(http.MethodGet, "/api/v1/users/1/brag-export?format=json", nil)
	wForbid := httptest.NewRecorder()
	otherRouter.ServeHTTP(wForbid, reqForbid)
	if wForbid.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-admin exporting another user, got %d", wForbid.Code)
	}

	// Admin exporting User 1's report -> 200 OK
	adminRouter := makeRouter(admin.ID, auth.RoleAdmin, h)
	reqAdmin, _ := http.NewRequest(http.MethodGet, "/api/v1/users/1/brag-export?format=pdf", nil)
	wAdmin := httptest.NewRecorder()
	adminRouter.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK || !bytes.HasPrefix(wAdmin.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("expected 200 OK PDF for admin, got %d", wAdmin.Code)
	}
	_ = u1
}

