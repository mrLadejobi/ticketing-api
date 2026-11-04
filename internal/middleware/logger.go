package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/response"
)

// StructuredLogger logs HTTP request details using standard log/slog.
func StructuredLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		reqID := response.GetRequestID(c)

		attrs := []slog.Attr{
			slog.String("request_id", reqID),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("query", rawQuery),
			slog.String("ip", clientIP),
			slog.Int("status", statusCode),
			slog.Duration("latency", latency),
			slog.Int("bytes", c.Writer.Size()),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("gin_errors", c.Errors.String()))
		}

		level := slog.LevelInfo
		if statusCode >= 500 {
			level = slog.LevelError
		} else if statusCode >= 400 {
			level = slog.LevelWarn
		}

		logger.LogAttrs(c.Request.Context(), level, "HTTP request handled", attrs...)
	}
}

