package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"ticketing-api/internal/response"
)

const HeaderXRequestID = "X-Request-ID"

// RequestID attaches a unique identifier to every request context and response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(HeaderXRequestID)
		if reqID == "" {
			reqID = uuid.NewString()
		}

		c.Set(response.RequestIDKey, reqID)
		c.Writer.Header().Set(HeaderXRequestID, reqID)
		c.Next()
	}
}

