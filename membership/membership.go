// Package membership provides reusable account and group workflow rules.
// Applications own their identity model, persistence, routes, and pages.
package membership

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidInput = errors.New("membership: invalid input")
	ErrSelfChange   = errors.New("membership: cannot change own account access")
)

type AccountAction string

const (
	Approve AccountAction = "approve"
	Disable AccountAction = "disable"
	Enable  AccountAction = "enable"
	SetRole AccountAction = "role"
)

// Store is implemented by the consumer's persistence adapter. Its account
// methods report whether the requested state transition was applied.
type Store interface {
	ApproveMember(context.Context, string) (bool, error)
	SetMemberDisabled(context.Context, string, bool) (bool, error)
	SetMemberRole(context.Context, string, string) (bool, error)
	CreateGroup(context.Context, string, string, string) (string, error)
	JoinGroup(context.Context, string, string) error
	LeaveGroup(context.Context, string, string) error
}

type Service struct {
	store Store
	roles map[string]struct{}
}

func New(store Store, allowedRoles ...string) (*Service, error) {
	if store == nil || len(allowedRoles) == 0 {
		return nil, ErrInvalidInput
	}
	roles := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		if role == "" || strings.TrimSpace(role) != role {
			return nil, ErrInvalidInput
		}
		roles[role] = struct{}{}
	}
	return &Service{store: store, roles: roles}, nil
}

// ChangeAccount applies an administrator-approved transition. The caller must
// authenticate and authorize the actor before calling this method.
func (s *Service) ChangeAccount(ctx context.Context, actorID, targetID string, action AccountAction, role string) (bool, error) {
	if actorID == "" || targetID == "" {
		return false, ErrInvalidInput
	}
	if actorID == targetID {
		return false, ErrSelfChange
	}
	switch action {
	case Approve:
		return s.store.ApproveMember(ctx, targetID)
	case Disable:
		return s.store.SetMemberDisabled(ctx, targetID, true)
	case Enable:
		return s.store.SetMemberDisabled(ctx, targetID, false)
	case SetRole:
		if _, ok := s.roles[role]; !ok {
			return false, ErrInvalidInput
		}
		return s.store.SetMemberRole(ctx, targetID, role)
	default:
		return false, ErrInvalidInput
	}
}

// CreateGroup normalizes and bounds the shared group description. The caller
// must authorize group creation before calling this method.
func (s *Service) CreateGroup(ctx context.Context, creatorID, name, description string) (string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if creatorID == "" || name == "" || description == "" || utf8.RuneCountInString(name) > 100 || utf8.RuneCountInString(description) > 500 {
		return "", ErrInvalidInput
	}
	return s.store.CreateGroup(ctx, creatorID, name, description)
}

// JoinGroup and LeaveGroup are idempotent when the consumer's store is
// idempotent. The caller must authenticate the member first.
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
