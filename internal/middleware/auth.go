package middleware

import (
	"context"
	"net/http"
	"strings"

	"ticketing-api/internal/auth"
	"ticketing-api/internal/response"

	"github.com/gin-gonic/gin"
)

type KeyValidator interface {
	ValidateKey(ctx context.Context, rawToken string) (*auth.User, error)
}

// Authenticate extracts and verifies JWT or Personal Access Token from either HttpOnly Cookie OR Bearer Authorization Header.
func Authenticate(tm *auth.TokenManager, kv ...KeyValidator) gin.HandlerFunc {
	var keyValidator KeyValidator
	if len(kv) > 0 {
		keyValidator = kv[0]
	}

	return func(c *gin.Context) {
		var tokenStr string

		// 1. First priority: Check HttpOnly Cookie (for secure browser frontends)
		if cookieToken, err := c.Cookie(auth.CookieAccessToken); err == nil && cookieToken != "" {
			tokenStr = cookieToken
		}

		// 2. Second priority: Fall back to Authorization: Bearer <token> (for CLI/Postman/PAT/mobile)
		if tokenStr == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// 3. If no token found in cookie or header
		if tokenStr == "" {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required. Please log in.")
			c.Abort()
			return
		}

		// 4. If token is a Personal Access Token (starts with pat_)
		if strings.HasPrefix(tokenStr, "pat_") && keyValidator != nil {
			user, err := keyValidator.ValidateKey(c.Request.Context(), tokenStr)
			if err != nil {
				response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired personal access token.")
				c.Abort()
				return
			}
			c.Set("user_id", user.ID)
			c.Set("user_email", user.Email)
			c.Set("user_role", string(user.Role))
			c.Next()
			return
		}

		// 5. Validate JWT Token signature and expiration
		claims, err := tm.ValidateToken(tokenStr)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired session. Please log in again.")
			c.Abort()
			return
		}

		// 6. Inject authenticated user identity into request context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", string(claims.Role))

		c.Next()
	}
}

// RequireRoles restricts access to specific user roles (RBAC).
func RequireRoles(allowedRoles ...auth.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("user_role")
		if !exists {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			c.Abort()
			return
		}

		userRole := auth.Role(roleVal.(string))
		for _, allowed := range allowedRoles {
			if userRole == allowed {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
		c.Abort()
	}
}
