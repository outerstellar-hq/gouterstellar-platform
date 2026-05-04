package model

import "encoding/json"

type SyncMessage struct {
	SyncID           string `json:"syncId"`
	Author           string `json:"author"`
	Content          string `json:"content"`
	UpdatedAtEpochMs int64  `json:"updatedAtEpochMs"`
	Deleted          bool   `json:"deleted"`
}

type SyncPushRequest struct {
	Messages []SyncMessage `json:"messages"`
}

type SyncConflict struct {
	SyncID        string       `json:"syncId"`
	Reason        string       `json:"reason"`
	ServerMessage *SyncMessage `json:"serverMessage"`
}

type SyncPushResponse struct {
	AppliedCount int            `json:"appliedCount"`
	Conflicts    []SyncConflict `json:"conflicts"`
}

type SyncPullResponse struct {
	Messages        []SyncMessage `json:"messages"`
	ServerTimestamp int64         `json:"serverTimestamp"`
}

type SyncStats struct {
	PushedCount   int `json:"pushedCount"`
	PulledCount   int `json:"pulledCount"`
	ConflictCount int `json:"conflictCount"`
}

type SyncContact struct {
	SyncID           string   `json:"syncId"`
	Name             string   `json:"name"`
	Emails           []string `json:"emails"`
	Phones           []string `json:"phones"`
	SocialMedia      []string `json:"socialMedia"`
	Company          string   `json:"company"`
	CompanyAddress   string   `json:"companyAddress"`
	Department       string   `json:"department"`
	UpdatedAtEpochMs int64    `json:"updatedAtEpochMs"`
	Deleted          bool     `json:"deleted"`
}

type SyncPushContactRequest struct {
	Contacts []SyncContact `json:"contacts"`
}

type SyncContactConflict struct {
	SyncID        string       `json:"syncId"`
	Reason        string       `json:"reason"`
	ServerContact *SyncContact `json:"serverContact"`
}

type SyncPushContactResponse struct {
	AppliedCount int                   `json:"appliedCount"`
	Conflicts    []SyncContactConflict `json:"conflicts"`
}

type SyncPullContactResponse struct {
	Contacts        []SyncContact `json:"contacts"`
	ServerTimestamp int64         `json:"serverTimestamp"`
}

func SyncMessageToJSON(msg SyncMessage) (string, error) {
	b, err := json.Marshal(msg)
	return string(b), err
}

func SyncMessageFromJSON(data string) (SyncMessage, error) {
	var msg SyncMessage
	err := json.Unmarshal([]byte(data), &msg)
	return msg, err
}

func SyncContactToJSON(c SyncContact) (string, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func SyncContactFromJSON(data string) (SyncContact, error) {
	var c SyncContact
	err := json.Unmarshal([]byte(data), &c)
	return c, err
}
