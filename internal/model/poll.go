package model

import (
	"time"

	"github.com/google/uuid"
)

type Poll struct {
	ID          int64      `json:"id"`
	SyncID      string     `json:"syncId"`
	CreatorID   uuid.UUID  `json:"creatorId"`
	Question    string     `json:"question"`
	MultiChoice bool       `json:"multiChoice"`
	ClosedAt    *time.Time `json:"closedAt"`
	Deadline    *time.Time `json:"deadline"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type PollOption struct {
	ID         int64  `json:"id"`
	PollID     int64  `json:"pollId"`
	Position   int16  `json:"position"`
	OptionText string `json:"optionText"`
}

type PollWithResults struct {
	Poll               Poll            `json:"poll"`
	Options            []PollOption    `json:"options"`
	VoteCounts         map[int64]int32 `json:"voteCounts"`
	TotalVotes         int32           `json:"totalVotes"`
	UserVotedOptionIDs []int64         `json:"userVotedOptionIds"`
}

type CreatePollRequest struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	MultiChoice bool     `json:"multiChoice"`
	Deadline    *string  `json:"deadline"`
}

type CreatePollInput struct {
	Question    string
	Options     []string
	MultiChoice bool
	Deadline    *time.Time
}

type CastPollVoteRequest struct {
	OptionID int64 `json:"optionId"`
}

type PollSummary struct {
	SyncID      string     `json:"syncId"`
	Question    string     `json:"question"`
	MultiChoice bool       `json:"multiChoice"`
	Closed      bool       `json:"closed"`
	Deadline    *time.Time `json:"deadline"`
	TotalVotes  int32      `json:"totalVotes"`
}

func (p Poll) IsClosed(now time.Time) bool {
	return p.ClosedAt != nil || (p.Deadline != nil && now.After(*p.Deadline))
}
