package viewmodel

import "net/url"

type MessagesPage struct {
	Messages   []MessageItem
	Pagination PaginationInfo
	Query      string
	Year       int
	Years      []int
	RefreshURL string
	Trash      bool
}

type SearchPage struct {
	Query       string
	Results     []SearchResult
	TypeFilter  string
	TypeFilters []SearchTypeFilter
}

type FooterStatus struct {
	Text string
}

type SidebarSelector struct {
	Heading    string
	Label      string
	ApplyLabel string
	Name       string
	Options    []SelectorOption
	Hidden     url.Values
	CSRFToken  string
}

type SelectorOption struct {
	Value    string
	Label    string
	Selected bool
}

type SearchResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Subtitle string  `json:"subtitle"`
	URL      string  `json:"url"`
	Type     string  `json:"type"`
	Score    float64 `json:"score"`
}

type SearchTypeFilter struct {
	Value  string
	Label  string
	URL    string
	Active bool
}

type TrashPage struct {
	MessageList  MessagesPage
	ContactList  ContactTrashList
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
	Language     string
}

type MessageEditPage struct {
	SyncID  string
	Author  string
	Content string
	Error   string
}

type MessageConflictPage struct {
	SyncID        string
	MyAuthor      string
	MyContent     string
	ServerAuthor  string
	ServerContent string
}

type VoteControls struct {
	SyncID       string
	Upvotes      int32
	Downvotes    int32
	NetScore     int32
	HasUpvoted   bool
	HasDownvoted bool
	CSRFToken    string
	Language     string
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
	Language      string
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
	RefreshURL string
}

// ContactDetailPage is the view model for the contact detail HTML page.
type ContactDetailPage struct {
	Contact ContactItem
	Form    ContactForm
}

type ContactForm struct {
	Editing   bool
	Contact   ContactItem
	CSRFToken string
	Language  string
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
	CSRFToken      string
	Language       string
}

type ContactTrashList struct {
	Contacts   []ContactItem
	Language   string
	RefreshURL string
}

type AuthPage struct {
	ReturnTo            string
	Username            string
	Error               string
	CSRFToken           string
	GoogleLoginEnabled  bool
	AppleLoginEnabled   bool
	RegistrationEnabled bool
	RegisterMode        bool
	TOTPRequired        bool
	PartialToken        string
}

type AdminUsersPage struct {
	Users      []UserItem
	Pagination PaginationInfo
}

type ExtensionsPage struct {
	Extensions []ExtensionCard
}

type ExtensionCard struct {
	ID             string
	Label          string
	Mode           string
	RouteCount     int
	MigrationCount int
	UIOwnership    []string
	APIOwnership   []string
	AdminOwnership []string
	AssetOwnership []string
	Routes         []ExtensionRoute
	Readiness      []ExtensionReadiness
}

type ExtensionRoute struct {
	Method      string
	PathPattern string
	Group       string
	Description string
	HandlerKind string
}

type ExtensionReadiness struct {
	Name    string
	Status  string
	Message string
}

type UserItem struct {
	ID                  string
	Username            string
	Email               string
	Role                string
	Enabled             bool
	FailedLoginAttempts int32
	IsLocked            bool
	IsSelf              bool
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

type NotificationBell struct {
	UnreadCount int64
	Language    string
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
	ThemeOptions             []SelectorOption
	LanguageOptions          []SelectorOption
	LayoutOptions            []SelectorOption
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
	PreviousURL string
	NextURL     string
	Language    string
}

type ErrorPage struct {
	StatusCode int
	Title      string
	Message    string
	RequestID  string
}

type ErrorHelp struct {
	Title string
	Items []string
}
