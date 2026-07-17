package viewmodel

import "github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo"

type ShellViewModel struct {
	Title                string
	User                 *UserContext
	Theme                string
	IsDark               bool
	Layout               string
	LayoutCSS            string
	Language             string
	CurrentPath          string
	CSRFToken            string
	Version              string
	Build                buildinfo.Info
	RequestID            string
	CSPNonce             string
	Body                 string
	BodyData             interface{}
	Page                 *PageInfo
	NavItems             []NavItem
	Banners              []Banner
	CustomCSS            string
	Messages             []ToastMessage
	ShowSearchForm       bool
	ShowNotificationBell bool
}

type UserContext struct {
	ID       string
	Username string
	Role     string
	IsAdmin  bool
}

type NavItem struct {
	Label     string
	URL       string
	Icon      string
	Active    bool
	AdminOnly bool
	Children  []NavItem
}

type PageInfo struct {
	Title       string
	Description string
}

type ToastMessage struct {
	Type    string
	Message string
}

type Banner struct {
	ID          string
	Title       string
	Body        string
	Severity    string
	Dismissible bool
	DismissURL  string
}
