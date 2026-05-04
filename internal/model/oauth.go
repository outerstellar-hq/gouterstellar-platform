package model

import "time"

type PasswordResetToken struct {
	ID        int64
	UserID    string
	Token     string
	ExpiresAt time.Time
	Used      bool
}

type OAuthUserInfo struct {
	Subject     string
	Email       *string
	DisplayName *string
}

type OAuthConnection struct {
	ID       int64
	UserID   string
	Provider string
	Subject  string
	Email    *string
}
