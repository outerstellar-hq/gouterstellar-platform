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
	FailedLoginAttempts       int32
	LockedUntil               *time.Time
	TOTPSecret                *string
	TOTPEnabled               bool
	TOTPBackupCodes           *string
	FailedTOTPAttempts        int32
}

type UserSummary struct {
	ID                  string     `json:"id"`
	Username            string     `json:"username"`
	Email               string     `json:"email"`
	Role                string     `json:"role"`
	Enabled             bool       `json:"enabled"`
	FailedLoginAttempts int32      `json:"failedLoginAttempts"`
	LockedUntil         *time.Time `json:"lockedUntil"`
}

func (u *User) ToSummary() UserSummary {
	return UserSummary{
		ID:                  u.ID.String(),
		Username:            u.Username,
		Email:               u.Email,
		Role:                string(u.Role),
		Enabled:             u.Enabled,
		FailedLoginAttempts: u.FailedLoginAttempts,
		LockedUntil:         u.LockedUntil,
	}
}
