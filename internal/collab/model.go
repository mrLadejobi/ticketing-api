package collab

import (
	"time"

	"ticketing-api/internal/auth"
)

type Comment struct {
	ID        int               `json:"id"`
	TicketID  int               `json:"ticket_id"`
	AuthorID  int               `json:"author_id"`
	Author    *auth.UserSummary `json:"author,omitempty"`
	Body      string            `json:"body"`
	Mentions  []string          `json:"mentions"`
	DeletedAt *time.Time        `json:"deleted_at,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type CreateCommentRequest struct {
	Body string `json:"body" binding:"required,min=1,max=10000"`
}

type UpdateCommentRequest struct {
	Body string `json:"body" binding:"required,min=1,max=10000"`
}

type Watcher struct {
	ID        int               `json:"id"`
	TicketID  int               `json:"ticket_id"`
	UserID    int               `json:"user_id"`
	User      *auth.UserSummary `json:"user,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type AddWatcherRequest struct {
	UserID *int `json:"user_id"` // Optional: defaults to authenticated user if omitted
}

type Attachment struct {
	ID          int               `json:"id"`
	TicketID    int               `json:"ticket_id"`
	UploaderID  int               `json:"uploader_id"`
	Uploader    *auth.UserSummary `json:"uploader,omitempty"`
	FileName    string            `json:"file_name"`
	FileSize    int64             `json:"file_size"`
	ContentType string            `json:"content_type"`
	FileURL     string            `json:"file_url"`
	CreatedAt   time.Time         `json:"created_at"`
}

