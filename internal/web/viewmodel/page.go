package viewmodel

type MessagesPage struct {
	Messages   []MessageItem
	Pagination PaginationInfo
	Query      string
	Year       int
	Years      []int
}

type TrashPage struct {
	Messages     []MessageItem
	Contacts     []ContactItem
	MessageTotal int64
	ContactTotal int64
	DeletedTotal int64
}

type MessageItem struct {
	SyncID       string
	Author       string
	Content      string
	UpdatedAt    string
	UpdatedLabel string
	Dirty        bool
	Version      int64
	HasConflict  bool
	Deleted      bool
	Upvotes      int32
	Downvotes    int32
	NetScore     int32
	HasUpvoted   bool
	HasDownvoted bool
	CSRFToken    string
}

type MessageEditPage struct {
	SyncID  string
	Author  string
	Content string
	Error   string
}

type VoteControls struct {
	SyncID       string
	Upvotes      int32
	Downvotes    int32
	NetScore     int32
	HasUpvoted   bool
	HasDownvoted bool
	CSRFToken    string
}

type PollCard struct {
	SyncID        string
	Question      string
	MultiChoice   bool
	Closed        bool
	DeadlineLabel string
	TotalVotes    int32
	Options       []PollOption
	CSRFToken     string
}

type PollOption struct {
	ID        int64
	Text      string
	VoteCount int32
	Percent   int32
	Selected  bool
	CanVote   bool
}

type ContactsPage struct {
	Contacts   []ContactItem
	Pagination PaginationInfo
	Query      string
}

// ContactDetailPage is the view model for the contact detail HTML page.
type ContactDetailPage struct {
	Contact ContactItem
}

type ContactItem struct {
	SyncID         string
	Name           string
	Emails         []string
	Phones         []string
	Social         []string
	Company        string
	CompanyAddress string
	Department     string
	UpdatedAt      string
	Dirty          bool
	Deleted        bool
}

type AuthPage struct {
	ReturnTo            string
	Username            string
	Error               string
	CSRFToken           string
	GoogleLoginEnabled  bool
	RegistrationEnabled bool
	RegisterMode        bool
	TOTPRequired        bool
	PartialToken        string
}

type AdminUsersPage struct {
	Users      []UserItem
	Pagination PaginationInfo
}

type UserItem struct {
	ID                  string
	Username            string
	Email               string
	Role                string
	Enabled             bool
	FailedLoginAttempts int32
	IsLocked            bool
}

type AdminAuditPage struct {
	Entries    []AuditItem
	Pagination PaginationInfo
}

type AuditItem struct {
	ID             int64
	ActorUsername  string
	TargetUsername string
	Action         string
	Detail         string
	CreatedAt      string
}

type NotificationsPage struct {
	Notifications []NotificationItem
	UnreadCount   int
}

type NotificationItem struct {
	ID        string
	Title     string
	Body      string
	Type      string
	Read      bool
	CreatedAt string
}

type SettingsPage struct {
	ActiveTab                string
	Profile                  ProfileData
	ApiKeys                  []ApiKeyItem
	Theme                    string
	Language                 string
	Layout                   string
	NewApiKey                string
	TOTPEnabled              bool
	TOTPRemainingBackupCodes int
	TOTPSetup                *TOTPSetupData
	TOTPBackupCodes          []string
	Error                    string
}

type TOTPSetupData struct {
	Secret    string
	QRDataURI string
}

// SettingsSessionsPage is the view model for the active-sessions management page.
type SettingsSessionsPage struct {
	Sessions []SessionItem
}

// SessionItem is a UI-facing summary of a single active session. MaskedTokenHash
// is a short prefix of the stored token hash, safe to render; TokenHash carries
// the full hash so the revoke form can address the session.
type SessionItem struct {
	TokenHash       string
	MaskedTokenHash string
	CreatedAt       string
	ExpiresAt       string
}

type ProfileData struct {
	Username                  string
	Email                     string
	AvatarURL                 string
	EmailNotificationsEnabled bool
	PushNotificationsEnabled  bool
}

type ApiKeyItem struct {
	ID        int64
	KeyPrefix string
	Name      string
	CreatedAt string
	LastUsed  string
}

type PaginationInfo struct {
	CurrentPage int
	TotalPages  int
	TotalItems  int64
	HasPrevious bool
	HasNext     bool
	PageSize    int
}

type ErrorPage struct {
	StatusCode int
	Title      string
	Message    string
	RequestID  string
}

type HomePage struct {
	MessageCount int64
	ContactCount int64
	UserCount    int64
}
