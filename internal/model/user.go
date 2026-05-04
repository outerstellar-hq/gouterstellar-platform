package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleUser  UserRole = "USER"
	RoleAdmin UserRole = "ADMIN"
)

type User struct {
	ID                        uuid.UUID
	Username                  string
	Email                     string
	PasswordHash              string
	Role                      UserRole
	Enabled                   bool
	LastActivityAt            *time.Time
	AvatarURL                 *string
	EmailNotificationsEnabled bool
	PushNotificationsEnabled  bool
	Language                  *string
	Theme                     *string
	Layout                    *string
}

type UserSummary struct {
	ID       string
	Username string
	Email    string
	Role     string
	Enabled  bool
}

func (u *User) ToSummary() UserSummary {
	return UserSummary{
		ID:       u.ID.String(),
		Username: u.Username,
		Email:    u.Email,
		Role:     string(u.Role),
		Enabled:  u.Enabled,
	}
}
