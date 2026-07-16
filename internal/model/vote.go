package model

// VoteDirection is the only value accepted by the message-voting API.
type VoteDirection int16

const (
	VoteDown VoteDirection = -1
	VoteUp   VoteDirection = 1
)

func (d VoteDirection) Valid() bool {
	return d == VoteDown || d == VoteUp
}

type VoteRequest struct {
	Direction VoteDirection `json:"direction"`
}

type VoteScore struct {
	MessageSyncID string         `json:"messageSyncId"`
	Upvotes       int32          `json:"upvotes"`
	Downvotes     int32          `json:"downvotes"`
	NetScore      int32          `json:"netScore"`
	UserVote      *VoteDirection `json:"userVote"`
}
