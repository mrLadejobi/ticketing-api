package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/response"
)

// Recovery catches panics, logs stack trace with slog, and returns a 500 JSON envelope.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				reqID := response.GetRequestID(c)

				logger.ErrorContext(c.Request.Context(), "Panic recovered in HTTP handler",
					slog.Any("panic", r),
					slog.String("request_id", reqID),
					slog.String("stack", stack),
				)

				response.Error(
					c,
					http.StatusInternalServerError,
					response.CodeInternal,
					"An unexpected internal error occurred. Please contact support with the request_id.",
				)
				c.Abort()
			}
		}()

		c.Next()
	}
}

