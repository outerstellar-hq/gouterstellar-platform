package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	SyncSchemaVersion    = 1
	SyncMaxAuthorLength  = 100
	SyncMaxContentLength = 500
	SyncMaxNameLength    = 200
	SyncMaxCompanyLength = 200
	SyncMaxAddressLength = 500
	SyncMaxEmailLength   = 255
	SyncMaxPhoneLength   = 50
	SyncMaxSocialLength  = 255
)

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

func (r SyncPushRequest) Validate() error {
	errors := make([]string, 0)
	for _, message := range r.Messages {
		if strings.TrimSpace(message.SyncID) == "" {
			errors = append(errors, "syncId must not be blank")
		}
		if strings.TrimSpace(message.Author) == "" {
			errors = append(errors, "author must not be blank")
		}
		if strings.TrimSpace(message.Content) == "" {
			errors = append(errors, "content must not be blank")
		}
		if utf8.RuneCountInString(message.Author) > SyncMaxAuthorLength {
			errors = append(errors, "author cannot exceed 100 characters")
		}
		if utf8.RuneCountInString(message.Content) > SyncMaxContentLength {
			errors = append(errors, "content cannot exceed 500 characters")
		}
	}
	if len(errors) > 0 {
		return &ValidationError{Errors: errors}
	}
	return nil
}

type SyncConflict struct {
	SyncID        string       `json:"syncId"`
	Reason        string       `json:"reason"`
	ServerMessage *SyncMessage `json:"serverMessage"`
	ClientMessage *SyncMessage `json:"clientMessage"`
}

type SyncPushResponse struct {
	AppliedCount  int            `json:"appliedCount"`
	Conflicts     []SyncConflict `json:"conflicts"`
	SchemaVersion int            `json:"schemaVersion"`
}

type SyncPullResponse struct {
	Messages        []SyncMessage `json:"messages"`
	ServerTimestamp int64         `json:"serverTimestamp"`
	HasMore         bool          `json:"hasMore"`
	SchemaVersion   int           `json:"schemaVersion"`
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

func (r SyncPushContactRequest) Validate() error {
	errors := make([]string, 0)
	for _, contact := range r.Contacts {
		if strings.TrimSpace(contact.SyncID) == "" {
			errors = append(errors, "syncId must not be blank")
		}
		if strings.TrimSpace(contact.Name) == "" {
			errors = append(errors, "name must not be blank")
		}
		if utf8.RuneCountInString(contact.Name) > SyncMaxNameLength {
			errors = append(errors, "name cannot exceed 200 characters")
		}
		if utf8.RuneCountInString(contact.Company) > SyncMaxCompanyLength {
			errors = append(errors, "company cannot exceed 200 characters")
		}
		if utf8.RuneCountInString(contact.CompanyAddress) > SyncMaxAddressLength {
			errors = append(errors, "companyAddress cannot exceed 500 characters")
		}
		if utf8.RuneCountInString(contact.Department) > SyncMaxCompanyLength {
			errors = append(errors, "department cannot exceed 200 characters")
		}
		for i, email := range contact.Emails {
			if utf8.RuneCountInString(email) > SyncMaxEmailLength {
				errors = append(errors, "email["+strconv.Itoa(i)+"] cannot exceed 255 characters")
			}
		}
		for i, phone := range contact.Phones {
			if utf8.RuneCountInString(phone) > SyncMaxPhoneLength {
				errors = append(errors, "phone["+strconv.Itoa(i)+"] cannot exceed 50 characters")
			}
		}
		for i, social := range contact.SocialMedia {
			if utf8.RuneCountInString(social) > SyncMaxSocialLength {
				errors = append(errors, "socialMedia["+strconv.Itoa(i)+"] cannot exceed 255 characters")
			}
		}
	}
	if len(errors) > 0 {
		return &ValidationError{Errors: errors}
	}
	return nil
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
	HasMore         bool          `json:"hasMore"`
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
