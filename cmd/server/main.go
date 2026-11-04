package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/config"
	"ticketing-api/internal/database"
	"ticketing-api/internal/middleware"
	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"
)

func main() {
	// 1. Initialize Production Structured Logger (slog JSON handler)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 2. Load Configuration
	cfg := config.Load()

	// 3. Setup Storage Repository
	var ticketRepo ticket.Repository

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Warn("PostgreSQL connection failed. Falling back to in-memory store for local development",
			slog.String("error", err.Error()),
		)
		ticketRepo = ticket.NewMemoryRepository()
	} else {
		defer db.Close()
		logger.Info("Connected to PostgreSQL successfully")

		if err := database.Migrate(db); err != nil {
			logger.Error("Database migration failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("Database migrations verified and applied")

		ticketRepo = ticket.NewPostgresRepository(db)
	}

	// 4. Assemble Domain Layer
	ticketService := ticket.NewService(ticketRepo)
	ticketHandler := ticket.NewHandler(ticketService)

	// 5. Initialize Gin Engine with Custom Middleware Pipeline
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)

	// Pipeline: RequestID -> StructuredLogger -> Recovery -> CORS
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())

	// Health Check
	router.GET("/health", func(c *gin.Context) {
		response.OK(c, gin.H{
			"status":  "healthy",
			"service": "ticketing-api",
		})
	})

	// Mount API v1
	v1 := router.Group("/api/v1")
	{
		ticketHandler.RegisterRoutes(v1)
	}

	// 6. Production HTTP Server with Graceful Shutdown
	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run server in background goroutine
	go func() {
		logger.Info("Starting HTTP server", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed unexpectedly", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// 7. Wait for interrupt signal (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutting down server gracefully...", slog.String("signal", sig.String()))

	// Context with 10s deadline for in-flight requests to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.String("error", err.Error()))
	} else {
		logger.Info("Server stopped cleanly. Goodbye!")
	}
}
