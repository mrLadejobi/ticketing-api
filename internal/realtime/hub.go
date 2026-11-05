package realtime

import (
	"sync"
)

type Event struct {
	Type      string `json:"type"` // e.g. "ticket.updated", "comment.created", "notification.new"
	Payload   any    `json:"payload"`
	ProjectID *int   `json:"project_id,omitempty"`
	UserID    *int   `json:"user_id,omitempty"`
}

type Client struct {
	UserID    int
	ProjectID *int
	Send      chan Event
	Done      chan struct{}
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	userMap    map[int]map[*Client]bool
	projectMap map[int]map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		userMap:    make(map[int]map[*Client]bool),
		projectMap: make(map[int]map[*Client]bool),
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[c] = true

	// Register user mapping
	if h.userMap[c.UserID] == nil {
		h.userMap[c.UserID] = make(map[*Client]bool)
	}
	h.userMap[c.UserID][c] = true

	// Register project mapping if subscribed
	if c.ProjectID != nil {
		if h.projectMap[*c.ProjectID] == nil {
			h.projectMap[*c.ProjectID] = make(map[*Client]bool)
		}
		h.projectMap[*c.ProjectID][c] = true
	}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		if h.userMap[c.UserID] != nil {
			delete(h.userMap[c.UserID], c)
			if len(h.userMap[c.UserID]) == 0 {
				delete(h.userMap, c.UserID)
			}
		}
		if c.ProjectID != nil && h.projectMap[*c.ProjectID] != nil {
			delete(h.projectMap[*c.ProjectID], c)
			if len(h.projectMap[*c.ProjectID]) == 0 {
				delete(h.projectMap, *c.ProjectID)
			}
		}
		close(c.Done)
	}
}

func (h *Hub) Broadcast(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 1. If targeted to a single user
	if event.UserID != nil {
		if clients, ok := h.userMap[*event.UserID]; ok {
			for c := range clients {
				select {
				case c.Send <- event:
				default:
				}
			}
		}
		return
	}

	// 2. If targeted to a project
	if event.ProjectID != nil {
		if clients, ok := h.projectMap[*event.ProjectID]; ok {
			for c := range clients {
				select {
				case c.Send <- event:
				default:
				}
			}
		}
		return
	}

	// 3. System-wide broadcast
	for c := range h.clients {
		select {
		case c.Send <- event:
		default:
		}
	}
}

func (h *Hub) BroadcastToUser(userID int, event Event) {
	event.UserID = &userID
	h.Broadcast(event)
}

func (h *Hub) BroadcastToProject(projectID int, event Event) {
	event.ProjectID = &projectID
	h.Broadcast(event)
}

func (h *Hub) ActiveClientsCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
