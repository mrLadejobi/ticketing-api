package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ticketing-api/internal/activity"
	"ticketing-api/internal/apikey"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/collab"
	"ticketing-api/internal/config"
	"ticketing-api/internal/cycle"
	"ticketing-api/internal/database"
	"ticketing-api/internal/docs"
	"ticketing-api/internal/label"
	"ticketing-api/internal/metrics"
	"ticketing-api/internal/middleware"
	"ticketing-api/internal/notification"
	"ticketing-api/internal/project"
	"ticketing-api/internal/realtime"
	"ticketing-api/internal/relation"
	"ticketing-api/internal/response"
	"ticketing-api/internal/team"
	"ticketing-api/internal/ticket"
	"ticketing-api/internal/webhook"

	"github.com/gin-gonic/gin"
)

type notificationRealtimeBridge struct {
	hub *realtime.Hub
}

func (b *notificationRealtimeBridge) PublishUserNotification(userID int, n *notification.Notification) {
	b.hub.BroadcastToUser(userID, realtime.Event{
		Type:    "notification.new",
		Payload: n,
	})
}

type ticketActivityAdapter struct {
	service *activity.Service
}

func (a *ticketActivityAdapter) RecordTicketActivity(ctx context.Context, ticketID int, actorID *int, field, oldValue, newValue string) error {
	_, err := a.service.RecordTicketActivity(ctx, ticketID, actorID, field, oldValue, newValue)
	return err
}

type projectNotificationAdapter struct {
	projects      *project.Service
	notifications *notification.Service
	hub           *realtime.Hub
}

func (p *projectNotificationAdapter) NotifyProjectFeatureAdded(ctx context.Context, projectID, actorID, ticketID int, key, title string) {
	memberIDs, err := p.projects.ListMemberUserIDs(ctx, projectID)
	if err == nil {
		for _, uid := range memberIDs {
			_, _ = p.notifications.Notify(
				ctx, uid, &actorID, &ticketID,
				notification.TypeFeatureAdded,
				fmt.Sprintf("New Feature Added: %s", key),
				title,
			)
		}
	}
	if p.hub != nil {
		p.hub.BroadcastToProject(projectID, realtime.Event{
			Type:    "feature.created",
			Payload: map[string]any{"ticket_id": ticketID, "key": key, "title": title, "project_id": projectID},
		})
	}
}

func (p *projectNotificationAdapter) RecordProjectTicketLog(ctx context.Context, projectID int, actorID, ticketID *int, action, summary string) {
	_ = p.projects.RecordProjectLog(ctx, projectID, actorID, ticketID, action, summary)
}

func main() {
	// 1. Initialize Production Structured Logger with source file/line tracking
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	// 2. Load Configuration
	cfg := config.Load()

	// 3. Initialize Token Manager for JWT Authentication
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, 15*time.Minute, 7*24*time.Hour)

	// 4. Setup Storage Repositories (PostgreSQL with In-Memory dev fallback)
	var (
		authRepo         auth.Repository
		teamRepo         team.Repository
		projectRepo      project.Repository
		ticketRepo       ticket.Repository
		collabRepo       collab.Repository
		relationRepo     relation.Repository
		labelRepo        label.Repository
		cycleRepo        cycle.Repository
		activityRepo     activity.Repository
		apiKeyRepo       apikey.Repository
		notificationRepo notification.Repository
	)

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Warn("PostgreSQL connection failed. Falling back to in-memory store for local development",
			slog.String("error", err.Error()),
		)
		memAuth := auth.NewMemoryRepository()
		authRepo = memAuth
		teamRepo = team.NewMemoryRepository(memAuth)
		projectRepo = project.NewMemoryRepository(memAuth)
		ticketRepo = ticket.NewMemoryRepository(memAuth)
		collabRepo = collab.NewMemoryRepository(memAuth)
		relationRepo = relation.NewMemoryRepository(ticketRepo)
		labelRepo = label.NewMemoryRepository()
		cycleRepo = cycle.NewMemoryRepository()
		activityRepo = activity.NewMemoryRepository(memAuth)
		apiKeyRepo = apikey.NewMemoryRepository()
		notificationRepo = notification.NewMemoryRepository(memAuth)
	} else {
		defer db.Close()
		logger.Info("Connected to PostgreSQL successfully")

		if err := database.Migrate(db); err != nil {
			logger.Error("Database migration failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("Database migrations verified and applied")

		authRepo = auth.NewPostgresRepository(db)
		teamRepo = team.NewPostgresRepository(db)
		projectRepo = project.NewPostgresRepository(db)
		ticketRepo = ticket.NewPostgresRepository(db)
		collabRepo = collab.NewPostgresRepository(db)
		relationRepo = relation.NewPostgresRepository(db)
		labelRepo = label.NewPostgresRepository(db)
		cycleRepo = cycle.NewPostgresRepository(db)
		activityRepo = activity.NewPostgresRepository(db)
		apiKeyRepo = apikey.NewPostgresRepository(db)
		notificationRepo = notification.NewPostgresRepository(db)
	}

	// 5. Assemble Domain Services & Cross-Module Dependencies
	authService := auth.NewService(authRepo, tokenManager)
	authHandler := auth.NewHandler(authService, false)

	apiKeyService := apikey.NewService(apiKeyRepo, authRepo)
	apiKeyHandler := apikey.NewHandler(apiKeyService)

	authMiddleware := middleware.Authenticate(tokenManager, apiKeyService)
	adminMiddleware := middleware.RequireRoles(auth.RoleAdmin)

	teamService := team.NewService(teamRepo)
	teamHandler := team.NewHandler(teamService)

	projectService := project.NewService(projectRepo)

	ticketService := ticket.NewService(ticketRepo, projectService)
	ticketHandler := ticket.NewHandler(ticketService)

	projectHandler := project.NewHandler(projectService, ticketService)

	storageProvider, _ := collab.NewLocalDiskStorage("./uploads", "/uploads")
	collabService := collab.NewService(collabRepo, storageProvider)
	collabHandler := collab.NewHandler(collabService, ticketService)

	relationService := relation.NewService(relationRepo)
	relationHandler := relation.NewHandler(relationService, ticketService)

	labelService := label.NewService(labelRepo)
	labelHandler := label.NewHandler(labelService, projectService, ticketService)

	cycleService := cycle.NewService(cycleRepo, ticketService)
	cycleHandler := cycle.NewHandler(cycleService, projectService)

	activityService := activity.NewService(activityRepo)
	activityHandler := activity.NewHandler(activityService, ticketService)
	ticketService.SetActivityRecorder(&ticketActivityAdapter{service: activityService})
	if db != nil {
		ticketService.SetTxManager(database.NewTxManager(db))
	}

	var deliveryTracker webhook.DeliveryTracker
	if db != nil {
		deliveryTracker = webhook.NewPostgresDeliveryTracker(db)
	} else {
		deliveryTracker = webhook.NewMemoryDeliveryTracker()
	}
	webhookHandler := webhook.NewHandler(os.Getenv("GIT_WEBHOOK_SECRET"), ticketService, collabService, deliveryTracker)

	// Real-Time SSE Hub & Notifications
	sseHub := realtime.NewHub()
	sseHandler := realtime.NewHandler(sseHub)

	notificationService := notification.NewService(notificationRepo, &notificationRealtimeBridge{hub: sseHub})
	notificationHandler := notification.NewHandler(notificationService)

	ticketService.SetProjectNotifier(&projectNotificationAdapter{
		projects:      projectService,
		notifications: notificationService,
		hub:           sseHub,
	})

	metricsHandler := metrics.NewHandler(ticketService, authRepo, projectService)

	// 6. Initialize Gin Engine with Custom Middleware Pipeline
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)

	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())

	// Static file serving for ticket attachments
	router.Static("/uploads", "./uploads")

	// Root Endpoint
	router.GET("/", func(c *gin.Context) {
		response.OK(c, gin.H{
			"message": "Enterprise Ticketing API is online and healthy!",
			"version": "2.0.0",
			"docs": gin.H{
				"scalar_ui":   "/docs",
				"swagger_ui":  "/swagger",
				"openapi_raw": "/openapi.json",
			},
			"resources": gin.H{
				"auth":          "/api/v1/auth",
				"users":         "/api/v1/users",
				"teams":         "/api/v1/teams",
				"projects":      "/api/v1/projects",
				"tickets":       "/api/v1/tickets",
				"comments":      "/api/v1/comments",
				"relations":     "/api/v1/tickets/:id_or_key/relations",
				"labels":        "/api/v1/projects/:id/labels",
				"cycles":        "/api/v1/projects/:id/cycles",
				"activity":      "/api/v1/tickets/:id_or_key/activity",
				"audit_logs":    "/api/v1/audit-logs",
				"api_keys":      "/api/v1/api-keys",
				"webhooks":      "/api/v1/webhooks/git",
				"notifications": "/api/v1/notifications",
				"events_stream": "/api/v1/events/stream",
			},
		})
	})

	// Register Documentation Endpoints
	docs.RegisterDocRoutes(router)

	// Health Check (standard and v1 alias)
	healthHandler := func(c *gin.Context) {
		response.OK(c, gin.H{
			"status":  "healthy",
			"service": "ticketing-api",
		})
	}
	router.GET("/health", healthHandler)
	router.GET("/api/v1/health", healthHandler)

	// Mount API v1 Routes
	v1 := router.Group("/api/v1")
	{
		authHandler.RegisterRoutes(v1, authMiddleware, adminMiddleware)
		apiKeyHandler.RegisterRoutes(v1, authMiddleware)
		teamHandler.RegisterRoutes(v1, authMiddleware)
		projectHandler.RegisterRoutes(v1, authMiddleware)
		ticketHandler.RegisterRoutes(v1, authMiddleware, adminMiddleware)
		collabHandler.RegisterRoutes(v1, authMiddleware)
		relationHandler.RegisterRoutes(v1, authMiddleware)
		labelHandler.RegisterRoutes(v1, authMiddleware)
		cycleHandler.RegisterRoutes(v1, authMiddleware)
		activityHandler.RegisterRoutes(v1, authMiddleware, adminMiddleware)
		webhookHandler.RegisterRoutes(v1)
		notificationHandler.RegisterRoutes(v1, authMiddleware)
		sseHandler.RegisterRoutes(v1, authMiddleware)
		metricsHandler.RegisterRoutes(v1, authMiddleware)
	}

	// 7. Production HTTP Server with Graceful Shutdown
	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("Starting HTTP server", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed unexpectedly", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutting down server gracefully...", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.String("error", err.Error()))
	} else {
		logger.Info("Server stopped cleanly. Goodbye!")
	}
}
