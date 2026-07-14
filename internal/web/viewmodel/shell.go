package viewmodel

type ShellViewModel struct {
	Title     string
	User      *UserContext
	Theme     string
	IsDark    bool
	Layout    string
	Language  string
	CSRFToken string
	Version   string
	RequestID string
	Body      string
	BodyData  interface{}
	Page      *PageInfo
	NavItems  []NavItem
	CustomCSS string
	Messages  []ToastMessage
}

type UserContext struct {
	ID       string
	Username string
	Role     string
	IsAdmin  bool
}

type NavItem struct {
	Label    string
	URL      string
	Icon     string
	Active   bool
	Children []NavItem
}

type PageInfo struct {
	Title       string
	Description string
}

type ToastMessage struct {
	Type    string
	Message string
}
