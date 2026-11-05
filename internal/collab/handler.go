package collab

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/auth"
	"ticketing-api/internal/response"
	"ticketing-api/internal/ticket"
)

type TicketResolver interface {
	GetByIDOrKey(ctx context.Context, idOrKey string) (*ticket.Ticket, error)
}

type Handler struct {
	service        *Service
	ticketResolver TicketResolver
}

func NewHandler(service *Service, ticketResolver TicketResolver, uploadDir ...string) *Handler {
	return &Handler{
		service:        service,
		ticketResolver: ticketResolver,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Nested routes under /tickets/:id_or_key/...
	ticketGroup := rg.Group("/tickets/:id_or_key")
	ticketGroup.Use(authMiddleware)
	{
		// Comments
		ticketGroup.GET("/comments", h.ListComments)
		ticketGroup.POST("/comments", h.CreateComment)

		// Watchers
		ticketGroup.GET("/watchers", h.ListWatchers)
		ticketGroup.POST("/watchers", h.AddWatcher)
		ticketGroup.DELETE("/watchers", h.RemoveSelfWatcher)
		ticketGroup.DELETE("/watchers/:user_id", h.RemoveWatcher)

		// Attachments
		ticketGroup.GET("/attachments", h.ListAttachments)
		ticketGroup.POST("/attachments", h.UploadAttachment)
	}

	// Standalone Comment mutations
	commentGroup := rg.Group("/comments")
	commentGroup.Use(authMiddleware)
	{
		commentGroup.PATCH("/:comment_id", h.UpdateComment)
		commentGroup.DELETE("/:comment_id", h.DeleteComment)
	}
}

func (h *Handler) resolveTicketID(c *gin.Context) (int, error) {
	idOrKey := c.Param("id_or_key")
	t, err := h.ticketResolver.GetByIDOrKey(c.Request.Context(), idOrKey)
	if err != nil {
		return 0, err
	}
	return t.ID, nil
}

// CreateComment handles POST /api/v1/tickets/:id_or_key/comments
func (h *Handler) CreateComment(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	userID := c.GetInt("user_id")

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	comment, err := h.service.CreateComment(c.Request.Context(), ticketID, userID, req.Body)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to create comment")
		return
	}

	response.Created(c, comment)
}

// ListComments handles GET /api/v1/tickets/:id_or_key/comments
func (h *Handler) ListComments(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	comments, err := h.service.ListComments(c.Request.Context(), ticketID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to retrieve comments")
		return
	}

	response.OK(c, comments)
}

// UpdateComment handles PATCH /api/v1/comments/:comment_id
func (h *Handler) UpdateComment(c *gin.Context) {
	commentID, err := strconv.Atoi(c.Param("comment_id"))
	if err != nil || commentID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid comment ID")
		return
	}

	userID := c.GetInt("user_id")
	userRole := auth.Role(c.GetString("user_role"))

	var req UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	comment, err := h.service.UpdateComment(c.Request.Context(), commentID, userID, userRole, req.Body)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "Only the author or an admin can edit this comment")
			return
		}
		if errors.Is(err, ErrCommentNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Comment not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to update comment")
		return
	}

	response.OK(c, comment)
}

// DeleteComment handles DELETE /api/v1/comments/:comment_id (Soft Delete)
func (h *Handler) DeleteComment(c *gin.Context) {
	commentID, err := strconv.Atoi(c.Param("comment_id"))
	if err != nil || commentID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid comment ID")
		return
	}

	userID := c.GetInt("user_id")
	userRole := auth.Role(c.GetString("user_role"))

	if err := h.service.DeleteComment(c.Request.Context(), commentID, userID, userRole); err != nil {
		if errors.Is(err, ErrForbidden) {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "Only the author or an admin can delete this comment")
			return
		}
		if errors.Is(err, ErrCommentNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Comment not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to delete comment")
		return
	}

	response.OK(c, gin.H{"deleted": true, "comment_id": commentID})
}

// AddWatcher handles POST /api/v1/tickets/:id_or_key/watchers
func (h *Handler) AddWatcher(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	targetUserID := c.GetInt("user_id")

	var req AddWatcherRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.UserID != nil && *req.UserID > 0 {
		targetUserID = *req.UserID
	}

	w, err := h.service.AddWatcher(c.Request.Context(), ticketID, targetUserID)
	if err != nil {
		if errors.Is(err, ErrWatcherDuplicate) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "User is already watching this ticket")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to add watcher")
		return
	}

	response.Created(c, w)
}

// RemoveSelfWatcher handles DELETE /api/v1/tickets/:id_or_key/watchers
func (h *Handler) RemoveSelfWatcher(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	userID := c.GetInt("user_id")
	if err := h.service.RemoveWatcher(c.Request.Context(), ticketID, userID); err != nil {
		if errors.Is(err, ErrWatcherNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "You are not watching this ticket")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to remove watcher")
		return
	}

	response.OK(c, gin.H{"unwatched": true, "ticket_id": ticketID, "user_id": userID})
}

// RemoveWatcher handles DELETE /api/v1/tickets/:id_or_key/watchers/:user_id
func (h *Handler) RemoveWatcher(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	targetUserID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || targetUserID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid user ID")
		return
	}

	if err := h.service.RemoveWatcher(c.Request.Context(), ticketID, targetUserID); err != nil {
		if errors.Is(err, ErrWatcherNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "User is not watching this ticket")
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to remove watcher")
		return
	}

	response.OK(c, gin.H{"unwatched": true, "ticket_id": ticketID, "user_id": targetUserID})
}

// ListWatchers handles GET /api/v1/tickets/:id_or_key/watchers
func (h *Handler) ListWatchers(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	watchers, err := h.service.ListWatchers(c.Request.Context(), ticketID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to list watchers")
		return
	}

	response.OK(c, watchers)
}

// UploadAttachment handles POST /api/v1/tickets/:id_or_key/attachments
func (h *Handler) UploadAttachment(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	userID := c.GetInt("user_id")

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "File upload missing ('file' form field required)")
		return
	}

	f, err := file.Open()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to open uploaded file")
		return
	}
	defer f.Close()

	saved, err := h.service.SaveAttachment(
		c.Request.Context(),
		ticketID,
		userID,
		file.Filename,
		file.Header.Get("Content-Type"),
		file.Size,
		f,
	)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to save attachment")
		return
	}

	response.Created(c, saved)
}

// ListAttachments handles GET /api/v1/tickets/:id_or_key/attachments
func (h *Handler) ListAttachments(c *gin.Context) {
	ticketID, err := h.resolveTicketID(c)
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ticket not found")
		return
	}

	attachments, err := h.service.ListAttachments(c.Request.Context(), ticketID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to list attachments")
		return
	}

	response.OK(c, attachments)
}

