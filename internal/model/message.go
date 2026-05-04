package model

import "time"

type StoredMessage struct {
	SyncID           string
	Author           string
	Content          string
	UpdatedAtEpochMs int64
	Dirty            bool
	Deleted          bool
	Version          int64
	SyncConflict     *string
}

type MessageSummary struct {
	SyncID           string
	Author           string
	Content          string
	UpdatedAtEpochMs int64
	Dirty            bool
	Version          int64
	HasConflict      bool
}

func (m *MessageSummary) UpdatedAtLabel() string {
	return time.UnixMilli(m.UpdatedAtEpochMs).Format("2006-01-02 15:04")
}
