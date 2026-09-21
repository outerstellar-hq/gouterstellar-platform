// Package group provides reusable group membership workflow rules.
// Applications own group storage, authorization, routes, and presentation.
package group

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidInput = errors.New("group: invalid input")

type Store interface {
	CreateGroup(context.Context, string, string, string) (string, error)
	JoinGroup(context.Context, string, string) error
	LeaveGroup(context.Context, string, string) error
}

type Service struct{ store Store }

func New(store Store) (*Service, error) {
	if store == nil {
		return nil, ErrInvalidInput
	}
	return &Service{store: store}, nil
}

// CreateGroup requires the caller to authorize group creation.
func (s *Service) CreateGroup(ctx context.Context, creatorID, name, description string) (string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if creatorID == "" || name == "" || description == "" || utf8.RuneCountInString(name) > 100 || utf8.RuneCountInString(description) > 500 {
		return "", ErrInvalidInput
	}
	return s.store.CreateGroup(ctx, creatorID, name, description)
}

// JoinGroup and LeaveGroup are idempotent when the application store is.
func (s *Service) JoinGroup(ctx context.Context, groupID, memberID string) error {
	if groupID == "" || memberID == "" {
		return ErrInvalidInput
	}
	return s.store.JoinGroup(ctx, groupID, memberID)
}

func (s *Service) LeaveGroup(ctx context.Context, groupID, memberID string) error {
	if groupID == "" || memberID == "" {
		return ErrInvalidInput
	}
	return s.store.LeaveGroup(ctx, groupID, memberID)
}
