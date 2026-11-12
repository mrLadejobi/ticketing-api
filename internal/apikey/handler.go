package apikey

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
	keys := rg.Group("/api-keys", authMiddleware)
	{
		keys.POST("", h.CreateKey)
		keys.GET("", h.ListKeys)
		keys.DELETE("/:id", h.DeleteKey)
	}
}

func (h *Handler) CreateKey(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	userID := userIDVal.(int)

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	created, err := h.service.CreateKey(c.Request.Context(), userID, req)
	if err != nil {
		response.InternalError(c, err, err.Error())
		return
	}

	response.Created(c, created)
}

func (h *Handler) ListKeys(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	userID := userIDVal.(int)

	keys, err := h.service.ListKeys(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, err, err.Error())
		return
	}

	if keys == nil {
		keys = make([]APIKey, 0)
	}

	response.OK(c, keys)
}

func (h *Handler) DeleteKey(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	userID := userIDVal.(int)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid API key ID")
		return
	}

	if err := h.service.DeleteKey(c.Request.Context(), id, userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "API key not found")
			return
		}
		response.InternalError(c, err, err.Error())
		return
	}

	response.OK(c, gin.H{"deleted": true, "key_id": id})
}

