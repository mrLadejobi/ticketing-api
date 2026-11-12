package auth

import (
	"errors"
	"net/http"
	"strconv"

	"ticketing-api/internal/response"

	"github.com/gin-gonic/gin"
)

const (
	CookieAccessToken  = "access_token"
	CookieRefreshToken = "refresh_token"
	AccessMaxAge       = 15 * 60          // 15 minutes
	RefreshMaxAge      = 7 * 24 * 60 * 60 // 7 days
)

type Handler struct {
	service        *Service
	isCookieSecure bool
}

func NewHandler(service *Service, isCookieSecure bool) *Handler {
	return &Handler{
		service:        service,
		isCookieSecure: isCookieSecure,
	}
}

// RegisterRoutes registers all authentication and user management routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware, adminMiddleware gin.HandlerFunc) {
	authGroup := rg.Group("/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)
		authGroup.POST("/refresh", h.Refresh)
		authGroup.POST("/logout", h.Logout)
		authGroup.GET("/me", authMiddleware, h.Me)
	}

	usersGroup := rg.Group("/users")
	usersGroup.Use(authMiddleware)
	{
		usersGroup.GET("", h.ListUsers)
		usersGroup.PATCH("/:id/role", adminMiddleware, h.UpdateRole)
	}
}

// Register handles POST /api/v1/auth/register
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	authRes, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "An account with this email already exists")
			return
		}
		response.InternalError(c, err, "Registration failed")
		return
	}

	h.setAccessCookie(c, authRes.AccessToken, AccessMaxAge)
	h.setRefreshCookie(c, authRes.RefreshToken, RefreshMaxAge)

	response.Created(c, gin.H{
		"user":          authRes.User,
		"access_token":  authRes.AccessToken,
		"refresh_token": authRes.RefreshToken,
	})
}

// Login handles POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	authRes, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid email or password")
			return
		}
		response.InternalError(c, err, "Login failed")
		return
	}

	h.setAccessCookie(c, authRes.AccessToken, AccessMaxAge)
	h.setRefreshCookie(c, authRes.RefreshToken, RefreshMaxAge)

	response.OK(c, gin.H{
		"user":          authRes.User,
		"access_token":  authRes.AccessToken,
		"refresh_token": authRes.RefreshToken,
	})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *Handler) Refresh(c *gin.Context) {
	var refreshTokenStr string

	if cookieToken, err := c.Cookie(CookieRefreshToken); err == nil && cookieToken != "" {
		refreshTokenStr = cookieToken
	}

	if refreshTokenStr == "" {
		var req RefreshTokenRequest
		if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
			refreshTokenStr = req.RefreshToken
		}
	}

	if refreshTokenStr == "" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Refresh token missing. Please log in.")
		return
	}

	newPair, user, err := h.service.RefreshToken(c.Request.Context(), refreshTokenStr)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired refresh token. Please log in again.")
		return
	}

	h.setAccessCookie(c, newPair.AccessToken, AccessMaxAge)
	h.setRefreshCookie(c, newPair.RefreshToken, RefreshMaxAge)

	response.OK(c, gin.H{
		"user":          user,
		"access_token":  newPair.AccessToken,
		"refresh_token": newPair.RefreshToken,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	h.setAccessCookie(c, "", -1)
	h.setRefreshCookie(c, "", -1)

	response.OK(c, gin.H{
		"message": "Successfully logged out and session terminated",
	})
}

// Me handles GET /api/v1/auth/me
func (h *Handler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	user, err := h.service.GetProfile(c.Request.Context(), userID.(int))
	if err != nil {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "User not found")
		return
	}

	response.OK(c, user)
}

// UpdateRole handles PATCH /api/v1/users/:id/role (Admin guarded)
func (h *Handler) UpdateRole(c *gin.Context) {
	targetID, err := strconv.Atoi(c.Param("id"))
	if err != nil || targetID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid user ID")
		return
	}

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	updatedUser, err := h.service.UpdateUserRole(c.Request.Context(), targetID, req.Role)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "User not found")
			return
		}
		response.InternalError(c, err, "Failed to update user role")
		return
	}

	response.OK(c, updatedUser)
}

// ListUsers handles GET /api/v1/users (returns user summaries for assignment)
func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.service.ListUsers(c.Request.Context())
	if err != nil {
		response.InternalError(c, err, "Failed to list users")
		return
	}

	response.OK(c, users)
}

func (h *Handler) setAccessCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		CookieAccessToken,
		token,
		maxAge,
		"/",
		"",
		h.isCookieSecure,
		true,
	)
}

func (h *Handler) setRefreshCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		CookieRefreshToken,
		token,
		maxAge,
		"/api/v1/auth",
		"",
		h.isCookieSecure,
		true,
	)
}
