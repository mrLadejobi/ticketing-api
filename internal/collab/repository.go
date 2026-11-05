package collab

import (
	"context"
	"errors"
	"sync"
	"time"

	"ticketing-api/internal/auth"
)

var (
	ErrCommentNotFound    = errors.New("comment not found")
	ErrWatcherNotFound    = errors.New("user is not watching this ticket")
	ErrWatcherDuplicate   = errors.New("user is already watching this ticket")
	ErrAttachmentNotFound = errors.New("attachment not found")
)

type Repository interface {
	CreateComment(ctx context.Context, c *Comment) (*Comment, error)
	GetCommentByID(ctx context.Context, id int) (*Comment, error)
	UpdateComment(ctx context.Context, id int, body string, mentions []string) (*Comment, error)
	SoftDeleteComment(ctx context.Context, id int) error
	ListCommentsByTicket(ctx context.Context, ticketID int) ([]Comment, error)

	AddWatcher(ctx context.Context, ticketID, userID int) (*Watcher, error)
	RemoveWatcher(ctx context.Context, ticketID, userID int) error
	ListWatchersByTicket(ctx context.Context, ticketID int) ([]Watcher, error)

	CreateAttachment(ctx context.Context, a *Attachment) (*Attachment, error)
	ListAttachmentsByTicket(ctx context.Context, ticketID int) ([]Attachment, error)
}

type MemoryRepository struct {
	mu          sync.RWMutex
	comments    map[int]*Comment
	watchers    map[int][]*Watcher    // ticketID -> watchers
	attachments map[int][]*Attachment // ticketID -> attachments
	nextComment int
	nextWatcher int
	nextAttach  int
	userRepo    auth.Repository
}

func NewMemoryRepository(userRepo auth.Repository) *MemoryRepository {
	return &MemoryRepository{
		comments:    make(map[int]*Comment),
		watchers:    make(map[int][]*Watcher),
		attachments: make(map[int][]*Attachment),
		nextComment: 1,
		nextWatcher: 1,
		nextAttach:  1,
		userRepo:    userRepo,
	}
}

func (r *MemoryRepository) CreateComment(ctx context.Context, c *Comment) (*Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	c.ID = r.nextComment
	c.CreatedAt = now
	c.UpdatedAt = now

	if r.userRepo != nil {
		if u, err := r.userRepo.GetUserByID(ctx, c.AuthorID); err == nil {
			s := u.Summary()
			c.Author = &s
		}
	}

	r.comments[c.ID] = c
	r.nextComment++
	return c, nil
}

func (r *MemoryRepository) GetCommentByID(ctx context.Context, id int) (*Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, exists := r.comments[id]
	if !exists || c.DeletedAt != nil {
		return nil, ErrCommentNotFound
	}
	return c, nil
}

func (r *MemoryRepository) UpdateComment(ctx context.Context, id int, body string, mentions []string) (*Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, exists := r.comments[id]
	if !exists || c.DeletedAt != nil {
		return nil, ErrCommentNotFound
	}

	c.Body = body
	c.Mentions = mentions
	c.UpdatedAt = time.Now().UTC()
	return c, nil
}

func (r *MemoryRepository) SoftDeleteComment(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, exists := r.comments[id]
	if !exists || c.DeletedAt != nil {
		return ErrCommentNotFound
	}

	now := time.Now().UTC()
	c.DeletedAt = &now
	c.UpdatedAt = now
	return nil
}

func (r *MemoryRepository) ListCommentsByTicket(ctx context.Context, ticketID int) ([]Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Comment, 0)
	for _, c := range r.comments {
		if c.TicketID == ticketID && c.DeletedAt == nil {
			list = append(list, *c)
		}
	}
	return list, nil
}

func (r *MemoryRepository) AddWatcher(ctx context.Context, ticketID, userID int) (*Watcher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.watchers[ticketID]
	for _, w := range list {
		if w.UserID == userID {
			return nil, ErrWatcherDuplicate
		}
	}

	w := &Watcher{
		ID:        r.nextWatcher,
		TicketID:  ticketID,
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	}
	r.nextWatcher++

	if r.userRepo != nil {
		if u, err := r.userRepo.GetUserByID(ctx, userID); err == nil {
			s := u.Summary()
			w.User = &s
		}
	}

	r.watchers[ticketID] = append(r.watchers[ticketID], w)
	return w, nil
}

func (r *MemoryRepository) RemoveWatcher(ctx context.Context, ticketID, userID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.watchers[ticketID]
	for i, w := range list {
		if w.UserID == userID {
			r.watchers[ticketID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return ErrWatcherNotFound
}

func (r *MemoryRepository) ListWatchersByTicket(ctx context.Context, ticketID int) ([]Watcher, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.watchers[ticketID]
	res := make([]Watcher, 0, len(list))
	for _, w := range list {
		res = append(res, *w)
	}
	return res, nil
}

func (r *MemoryRepository) CreateAttachment(ctx context.Context, a *Attachment) (*Attachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	a.ID = r.nextAttach
	a.CreatedAt = now
	r.nextAttach++

	if r.userRepo != nil {
		if u, err := r.userRepo.GetUserByID(ctx, a.UploaderID); err == nil {
			s := u.Summary()
			a.Uploader = &s
		}
	}

	r.attachments[a.TicketID] = append(r.attachments[a.TicketID], a)
	return a, nil
}

func (r *MemoryRepository) ListAttachmentsByTicket(ctx context.Context, ticketID int) ([]Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.attachments[ticketID]
	res := make([]Attachment, 0, len(list))
	for _, a := range list {
		res = append(res, *a)
	}
	return res, nil
}

