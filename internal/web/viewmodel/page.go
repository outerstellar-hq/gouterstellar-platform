package viewmodel

type MessagesPage struct {
	Messages   []MessageItem
	Pagination PaginationInfo
	Query      string
	Year       int
	Years      []int
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
}

type ContactsPage struct {
	Contacts   []ContactItem
	Pagination PaginationInfo
	Query      string
}

type ContactItem struct {
	SyncID    string
	Name      string
	Emails    []string
	Phones    []string
	Social    []string
	Company   string
	UpdatedAt string
	Dirty     bool
	Deleted   bool
}

type AuthPage struct {
	ReturnTo  string
	Error     string
	CSRFToken string
}

type AdminUsersPage struct {
	Users      []UserItem
	Pagination PaginationInfo
}

type UserItem struct {
	ID       string
	Username string
	Email    string
	Role     string
	Enabled  bool
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
	ActiveTab string
	Profile   ProfileData
	ApiKeys   []ApiKeyItem
	Theme     string
	Language  string
	Layout    string
	NewApiKey string
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

type TrashPage struct {
	Messages []MessageItem
	Contacts []ContactItem
}
