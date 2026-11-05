package notification

import (
	"errors"
	"net/http"
	"strconv"

	"ticketing-api/internal/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	nGroup := rg.Group("/notifications")
	nGroup.Use(authMiddleware)
	{
		nGroup.GET("", h.List)
		nGroup.PATCH("/:id/read", h.MarkRead)
		nGroup.PATCH("/read-all", h.MarkAllRead)
	}
}

func (h *Handler) List(c *gin.Context) {
	userID := c.GetInt("user_id")

	unreadOnly := c.Query("unread_only") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := ListNotificationsFilter{
		UserID:     userID,
		UnreadOnly: unreadOnly,
		Limit:      limit,
		Offset:     offset,
	}

	notifications, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to list notifications")
		return
	}

	response.Paginated(c, notifications, total, limit, offset)
}

func (h *Handler) MarkRead(c *gin.Context) {
	userID := c.GetInt("user_id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid notification ID")
		return
	}

	if err := h.service.MarkAsRead(c.Request.Context(), id, userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Notification not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to mark notification as read")
		return
	}

	response.OK(c, gin.H{"message": "Notification marked as read"})
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	userID := c.GetInt("user_id")

	if err := h.service.MarkAllAsRead(c.Request.Context(), userID); err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to mark all notifications as read")
		return
	}

	response.OK(c, gin.H{"message": "All notifications marked as read"})
}

