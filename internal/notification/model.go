package notification

import (
	"time"

	"ticketing-api/internal/auth"
)

type NotificationType string

const (
	TypeMention       NotificationType = "MENTION"
	TypeAssigned      NotificationType = "ASSIGNED"
	TypeStatusChanged NotificationType = "STATUS_CHANGED"
	TypeCommentAdded  NotificationType = "COMMENT_ADDED"
	TypeFeatureAdded  NotificationType = "FEATURE_ADDED"
)

type Notification struct {
	ID        int               `json:"id"`
	UserID    int               `json:"user_id"`
	ActorID   *int              `json:"actor_id,omitempty"`
	Actor     *auth.UserSummary `json:"actor,omitempty"`
	TicketID  *int              `json:"ticket_id,omitempty"`
	Type      NotificationType  `json:"type"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	IsRead    bool              `json:"is_read"`
	CreatedAt time.Time         `json:"created_at"`
}

type ListNotificationsFilter struct {
	UserID     int
	UnreadOnly bool
	Limit      int
	Offset     int
}

