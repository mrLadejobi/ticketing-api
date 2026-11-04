package response

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const (
	RequestIDKey = "RequestID"

	CodeInvalidArgument = "INVALID_ARGUMENT"
	CodeNotFound        = "NOT_FOUND"
	CodeInternal        = "INTERNAL_ERROR"
	CodeConflict        = "CONFLICT"
)

// StandardResponse is the unified top-level response envelope for all API endpoints.
type StandardResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
	Meta    any       `json:"meta,omitempty"`
}

// APIError represents structured error information.
type APIError struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	RequestID string       `json:"request_id,omitempty"`
	Details   []FieldError `json:"details,omitempty"`
}

// FieldError gives precise field-level feedback for failed validations.
type FieldError struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// PaginationMeta provides pagination metadata for list responses.
type PaginationMeta struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// OK sends an HTTP 200 response with data.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    data,
	})
}

// Created sends an HTTP 201 response with data.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, StandardResponse{
		Success: true,
		Data:    data,
	})
}

// Paginated sends an HTTP 200 response with a paginated collection and metadata.
func Paginated(c *gin.Context, data any, total, limit, offset int) {
	hasMore := (offset + limit) < total
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    data,
		Meta: PaginationMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: hasMore,
		},
	})
}

// Error sends a structured error response with status code, error code, and message.
func Error(c *gin.Context, httpStatus int, code, message string, details ...FieldError) {
	reqID := GetRequestID(c)
	c.JSON(httpStatus, StandardResponse{
		Success: false,
		Error: &APIError{
			Code:      code,
			Message:   message,
			RequestID: reqID,
			Details:   details,
		},
	})
}

// ValidationError inspects Gin's validator.ValidationErrors and produces human-friendly field errors.
func ValidationError(c *gin.Context, err error) {
	var valErrors validator.ValidationErrors
	if errors.As(err, &valErrors) {
		details := make([]FieldError, 0, len(valErrors))
		for _, fe := range valErrors {
			details = append(details, FieldError{
				Field: fe.Field(),
				Issue: formatValidationTag(fe),
			})
		}
		Error(c, http.StatusBadRequest, CodeInvalidArgument, "Invalid request payload", details...)
		return
	}

	// Fallback for non-validation binding errors (e.g. malformed JSON syntax)
	Error(c, http.StatusBadRequest, CodeInvalidArgument, "Malformed JSON or parameter: "+err.Error())
}

func formatValidationTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "this field is required"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must not exceed %s characters", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	default:
		return fmt.Sprintf("failed validation on rule '%s'", fe.Tag())
	}
}

// GetRequestID retrieves the request ID stored in the Gin context.
func GetRequestID(c *gin.Context) string {
	if val, exists := c.Get(RequestIDKey); exists {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

