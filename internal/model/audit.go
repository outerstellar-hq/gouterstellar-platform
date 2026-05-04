package model

import "time"

type AuditEntry struct {
	ID             int64     `json:"id"`
	ActorID        *string   `json:"actorId"`
	ActorUsername  *string   `json:"actorUsername"`
	TargetID       *string   `json:"targetId"`
	TargetUsername *string   `json:"targetUsername"`
	Action         string    `json:"action"`
	Detail         *string   `json:"detail"`
	CreatedAt      time.Time `json:"createdAt"`
}
