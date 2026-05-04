package model

import (
	"time"

	"github.com/google/uuid"
)

type OutboxEntry struct {
	ID          uuid.UUID
	PayloadType string
	Payload     string
	Status      string
	CreatedAt   time.Time
}
