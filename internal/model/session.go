package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        int64
	TokenHash string
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionLookup interface{ sessionLookup() }

type SessionActive struct{ User *User }

func (SessionActive) sessionLookup() {}

type SessionExpired struct{}

func (SessionExpired) sessionLookup() {}

type SessionNotFound struct{}

func (SessionNotFound) sessionLookup() {}
