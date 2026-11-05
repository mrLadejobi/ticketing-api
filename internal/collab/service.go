package collab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"ticketing-api/internal/auth"
)

var (
	ErrForbidden = errors.New("forbidden: you do not have permission to modify this resource")
	mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}|[a-zA-Z0-9_.-]+)`)
)

type Service struct {
	repo    Repository
	storage StorageProvider
}

func NewService(repo Repository, storage ...StorageProvider) *Service {
	var st StorageProvider
	if len(storage) > 0 {
		st = storage[0]
	} else {
		st = NewMemoryStorage()
	}
	return &Service{
		repo:    repo,
		storage: st,
	}
}

func (s *Service) SetStorageProvider(st StorageProvider) {
	s.storage = st
}

// ExtractMentions scans markdown text for @mentions or @emails
func ExtractMentions(body string) []string {
	matches := mentionRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return []string{}
	}

	seen := make(map[string]bool)
	var list []string
	for _, match := range matches {
		if len(match) > 1 {
			m := strings.TrimSpace(match[1])
			if m != "" && !seen[m] {
				seen[m] = true
				list = append(list, m)
			}
		}
	}
	return list
}

func (s *Service) CreateComment(ctx context.Context, ticketID, authorID int, body string) (*Comment, error) {
	mentions := ExtractMentions(body)
	c := &Comment{
		TicketID: ticketID,
		AuthorID: authorID,
		Body:     body,
		Mentions: mentions,
	}
	return s.repo.CreateComment(ctx, c)
}

func (s *Service) UpdateComment(ctx context.Context, commentID, requesterID int, requesterRole auth.Role, body string) (*Comment, error) {
	c, err := s.repo.GetCommentByID(ctx, commentID)
	if err != nil {
		return nil, err
	}

	// Only author or ADMIN can edit
	if c.AuthorID != requesterID && requesterRole != auth.RoleAdmin {
		return nil, ErrForbidden
	}

	mentions := ExtractMentions(body)
	return s.repo.UpdateComment(ctx, commentID, body, mentions)
}

func (s *Service) DeleteComment(ctx context.Context, commentID, requesterID int, requesterRole auth.Role) error {
	c, err := s.repo.GetCommentByID(ctx, commentID)
	if err != nil {
		return err
	}

	// Only author or ADMIN can delete
	if c.AuthorID != requesterID && requesterRole != auth.RoleAdmin {
		return ErrForbidden
	}

	return s.repo.SoftDeleteComment(ctx, commentID)
}

func (s *Service) ListComments(ctx context.Context, ticketID int) ([]Comment, error) {
	return s.repo.ListCommentsByTicket(ctx, ticketID)
}

func (s *Service) AddWatcher(ctx context.Context, ticketID, userID int) (*Watcher, error) {
	return s.repo.AddWatcher(ctx, ticketID, userID)
}

func (s *Service) RemoveWatcher(ctx context.Context, ticketID, userID int) error {
	return s.repo.RemoveWatcher(ctx, ticketID, userID)
}

func (s *Service) ListWatchers(ctx context.Context, ticketID int) ([]Watcher, error) {
	return s.repo.ListWatchersByTicket(ctx, ticketID)
}

func (s *Service) CreateAttachment(ctx context.Context, a *Attachment) (*Attachment, error) {
	return s.repo.CreateAttachment(ctx, a)
}

func (s *Service) SaveAttachment(ctx context.Context, ticketID, uploaderID int, filename, contentType string, size int64, r io.Reader) (*Attachment, error) {
	if s.storage == nil {
		s.storage = NewMemoryStorage()
	}
	url, err := s.storage.Save(ctx, filename, r)
	if err != nil {
		return nil, fmt.Errorf("failed to save attachment binary: %w", err)
	}

	att := &Attachment{
		TicketID:    ticketID,
		UploaderID:  uploaderID,
		FileName:    filename,
		FileSize:    size,
		ContentType: contentType,
		FileURL:     url,
	}

	return s.repo.CreateAttachment(ctx, att)
}

func (s *Service) ListAttachments(ctx context.Context, ticketID int) ([]Attachment, error) {
	return s.repo.ListAttachmentsByTicket(ctx, ticketID)
}

func (s *Service) AddSystemComment(ctx context.Context, ticketID int, body string) error {
	mentions := ExtractMentions(body)
	c := &Comment{
		TicketID: ticketID,
		AuthorID: 1,
		Body:     body,
		Mentions: mentions,
	}
	_, err := s.repo.CreateComment(ctx, c)
	return err
}

