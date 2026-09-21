// Package member provides reusable account and core profile rules.
// Applications own storage, authorization, and any extra profile fields.
package member

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidInput = errors.New("member: invalid input")
	ErrSelfChange   = errors.New("member: cannot change own account access")
)

type AccountAction string

const (
	Approve AccountAction = "approve"
	Disable AccountAction = "disable"
	Enable  AccountAction = "enable"
	SetRole AccountAction = "role"
)

// Profile contains fields shared by member profiles across applications.
// Applications can embed it in a profile with their own fields.
type Profile struct {
	DisplayName string
	Bio         string
	Public      bool
	HasAvatar   bool
}

// NormalizeProfile trims and checks the common editable fields.
func NormalizeProfile(profile Profile) (Profile, error) {
	profile.DisplayName = strings.TrimSpace(profile.DisplayName)
	profile.Bio = strings.TrimSpace(profile.Bio)
	if utf8.RuneCountInString(profile.DisplayName) > 80 || utf8.RuneCountInString(profile.Bio) > 500 || (profile.Public && profile.DisplayName == "") {
		return Profile{}, ErrInvalidInput
	}
	return profile, nil
}

type Store interface {
	ApproveMember(context.Context, string) (bool, error)
	SetMemberDisabled(context.Context, string, bool) (bool, error)
	SetMemberRole(context.Context, string, string) (bool, error)
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

// ChangeAccount requires an authenticated, authorized actor.
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
