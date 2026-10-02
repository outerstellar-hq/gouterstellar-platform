// Package faq provides application-neutral question-and-answer workflows.
// Consumers own authorization, persistence, routes, and presentation.
package faq

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid = errors.New("faq: invalid question, answer, or search")
	ErrMissing = errors.New("faq: entry not found")
)

type Entry struct {
	ID        string
	Question  string
	Answer    string
	Published bool
	UpdatedAt time.Time
}

// Store is supplied by the consumer. SearchPublished must exclude drafts.
type Store interface {
	ListFAQPublished(context.Context, string) ([]Entry, error)
	ListFAQAll(context.Context) ([]Entry, error)
	FAQByID(context.Context, string) (Entry, error)
	CreateFAQ(context.Context, Entry) (string, error)
	UpdateFAQ(context.Context, Entry) (bool, error)
	DeleteFAQ(context.Context, string) (bool, error)
}

type Service struct{ store Store }

func New(store Store) (*Service, error) {
	if store == nil {
		return nil, ErrInvalid
	}
	return &Service{store: store}, nil
}

// Search returns published entries. Empty text lists all published entries.
func (s *Service) Search(ctx context.Context, text string) ([]Entry, error) {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > 100 {
		return nil, ErrInvalid
	}
	return s.store.ListFAQPublished(ctx, text)
}

// All and ByID are for callers that have already authorized administration.
func (s *Service) All(ctx context.Context) ([]Entry, error) { return s.store.ListFAQAll(ctx) }

func (s *Service) ByID(ctx context.Context, id string) (Entry, error) {
	if id == "" {
		return Entry{}, ErrInvalid
	}
	return s.store.FAQByID(ctx, id)
}

// Save creates a new entry when ID is empty and updates an existing entry
// otherwise. Draft answers are valid but never returned by Search.
func (s *Service) Save(ctx context.Context, entry Entry) (string, error) {
	entry.Question = strings.TrimSpace(entry.Question)
	entry.Answer = strings.TrimSpace(entry.Answer)
	if entry.Question == "" || entry.Answer == "" || utf8.RuneCountInString(entry.Question) > 200 || utf8.RuneCountInString(entry.Answer) > 8000 {
		return "", ErrInvalid
	}
	if entry.ID == "" {
		return s.store.CreateFAQ(ctx, entry)
	}
	changed, err := s.store.UpdateFAQ(ctx, entry)
	if err != nil {
		return "", err
	}
	if !changed {
		return "", ErrMissing
	}
	return entry.ID, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalid
	}
	changed, err := s.store.DeleteFAQ(ctx, id)
	if err != nil {
		return err
	}
	if !changed {
		return ErrMissing
	}
	return nil
}
