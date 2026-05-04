package model

type StoredContact struct {
	SyncID           string
	Name             string
	Emails           []string
	Phones           []string
	SocialMedia      []string
	Company          string
	CompanyAddress   string
	Department       string
	UpdatedAtEpochMs int64
	Dirty            bool
	Deleted          bool
	Version          int64
	SyncConflict     *string
}

type ContactSummary struct {
	SyncID           string
	Name             string
	Emails           []string
	Phones           []string
	SocialMedia      []string
	Company          string
	CompanyAddress   string
	Department       string
	UpdatedAtEpochMs int64
	Dirty            bool
	Version          int64
	HasConflict      bool
}
