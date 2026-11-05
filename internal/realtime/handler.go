package realtime

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	rg.GET("/events/stream", authMiddleware, h.StreamEvents)
}

func (h *Handler) StreamEvents(c *gin.Context) {
	userID := c.GetInt("user_id")

	var projectID *int
	if pStr := c.Query("project_id"); pStr != "" {
		if pid, err := strconv.Atoi(pStr); err == nil {
			projectID = &pid
		}
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	client := &Client{
		UserID:    userID,
		ProjectID: projectID,
		Send:      make(chan Event, 64),
		Done:      make(chan struct{}),
	}

	h.hub.Register(client)
	defer h.hub.Unregister(client)

	// Send initial handshake event
	_, _ = fmt.Fprintf(c.Writer, "event: connected\ndata: {\"status\":\"connected\",\"user_id\":%d}\n\n", userID)
	c.Writer.Flush()

	// Heartbeat every 25 seconds as requested
	heartbeatTicker := time.NewTicker(25 * time.Second)
	defer heartbeatTicker.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-client.Send:
			if !ok {
				return false
			}
			payloadBytes, err := json.Marshal(event.Payload)
			if err != nil {
				return true
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(payloadBytes))
			return true

		case <-heartbeatTicker.C:
			// Server-Sent Events heartbeat ping comment
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			return true

		case <-c.Request.Context().Done():
			return false
		}
	})
}

