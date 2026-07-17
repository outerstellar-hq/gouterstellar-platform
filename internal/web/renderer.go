package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
	"github.com/outerstellar-hq/gouterstellar-platform/pkg/i18n"
	"github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo"
)

// Renderer renders HTML pages through a shared layout (base.html) and
// partials for HTMX fragment responses. Each page is pre-cloned from the
// base template set so its {{ define "content" }} block resolves cleanly.
type Renderer struct {
	mu       sync.RWMutex
	base     *template.Template
	funcs    template.FuncMap
	pages    map[string]*template.Template
	owners   map[string]string
	partials *template.Template
	version  string
}

// NewRenderer parses base.html + partials into a base set, then clones
// it for each page file. Pages are keyed by filename without extension.
func NewRenderer(templateFS fs.FS, funcs template.FuncMap, version string) (*Renderer, error) {
	// Parse base into the shared base set.
	baseBytes, err := fs.ReadFile(templateFS, "template/base.html")
	if err != nil {
		return nil, fmt.Errorf("read base.html: %w", err)
	}

	base := template.New("").Funcs(funcs)
	base, err = base.Parse(string(baseBytes))
	if err != nil {
		return nil, fmt.Errorf("parse base.html: %w", err)
	}

	// Parse partials into the base set so they're available in every clone.
	partialEntries, err := fs.ReadDir(templateFS, "template/partials")
	if err != nil {
		return nil, fmt.Errorf("read partials dir: %w", err)
	}
	for _, entry := range partialEntries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		content, err := fs.ReadFile(templateFS, "template/partials/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read partial %s: %w", entry.Name(), err)
		}
		base, err = base.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parse partial %s: %w", entry.Name(), err)
		}
	}

	// Clone for each page.
	pages := make(map[string]*template.Template)
	pageEntries, err := fs.ReadDir(templateFS, "template/pages")
	if err != nil {
		return nil, fmt.Errorf("read pages dir: %w", err)
	}
	for _, entry := range pageEntries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		pageName := strings.TrimSuffix(entry.Name(), ".html")
		content, err := fs.ReadFile(templateFS, "template/pages/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read page %s: %w", entry.Name(), err)
		}
		clone, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone for page %s: %w", pageName, err)
		}
		clone, err = clone.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parse page %s: %w", pageName, err)
		}
		pages[pageName] = clone
	}

	// Separate partials set for fragment rendering.
	partials := template.New("").Funcs(funcs)
	for _, entry := range partialEntries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		content, _ := fs.ReadFile(templateFS, "template/partials/"+entry.Name())
		partials, err = partials.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parse partials set %s: %w", entry.Name(), err)
		}
	}

	owners := make(map[string]string, len(pages))
	for pageName := range pages {
		owners[pageName] = "platform"
	}

	return &Renderer{
		base:     base,
		funcs:    funcs,
		pages:    pages,
		owners:   owners,
		partials: partials,
		version:  version,
	}, nil
}

// RegisterTemplates validates and parses an extension's page templates during
// platform assembly. Page names are global and collision errors name both
// owners; extension partials are scoped to that extension's page clones.
func (r *Renderer) RegisterTemplates(owner string, source fs.FS, pagesDir, partialsDir string) error {
	owner = strings.TrimSpace(owner)
	pagesDir = strings.TrimSpace(pagesDir)
	partialsDir = strings.TrimSpace(partialsDir)
	if owner == "" {
		return fmt.Errorf("template owner must not be empty")
	}
	if source == nil {
		return fmt.Errorf("extension %s template filesystem is nil", owner)
	}
	if pagesDir == "" {
		return fmt.Errorf("extension %s pages directory must not be empty", owner)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	extensionBase, err := r.base.Clone()
	if err != nil {
		return fmt.Errorf("extension %s clone shared shell: %w", owner, err)
	}

	if partialsDir != "" {
		entries, readErr := fs.ReadDir(source, partialsDir)
		if readErr != nil {
			return fmt.Errorf("extension %s read partials directory %s: %w", owner, partialsDir, readErr)
		}
		definitions := make(map[string]string)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
				continue
			}
			filePath := path.Join(partialsDir, entry.Name())
			content, readErr := fs.ReadFile(source, filePath)
			if readErr != nil {
				return fmt.Errorf("extension %s read partial %s: %w", owner, filePath, readErr)
			}
			probe, parseErr := template.New("").Funcs(r.funcs).Parse(string(content))
			if parseErr != nil {
				return fmt.Errorf("extension %s parse partial %s: %w", owner, filePath, parseErr)
			}
			for _, definition := range probe.Templates() {
				name := definition.Name()
				if name == "" {
					continue
				}
				if existingFile, exists := definitions[name]; exists {
					return fmt.Errorf("extension %s template %q is defined by both %s and %s", owner, name, existingFile, filePath)
				}
				if r.base.Lookup(name) != nil {
					return fmt.Errorf("extension %s template %q conflicts with owner platform", owner, name)
				}
				definitions[name] = filePath
			}
			if _, parseErr = extensionBase.Parse(string(content)); parseErr != nil {
				return fmt.Errorf("extension %s parse partial %s: %w", owner, filePath, parseErr)
			}
		}
	}

	pageEntries, err := fs.ReadDir(source, pagesDir)
	if err != nil {
		return fmt.Errorf("extension %s read pages directory %s: %w", owner, pagesDir, err)
	}
	pending := make(map[string]*template.Template)
	for _, entry := range pageEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		pageName := strings.TrimSuffix(entry.Name(), ".html")
		if existingOwner, exists := r.owners[pageName]; exists {
			return fmt.Errorf("extension %s page %q conflicts with owner %s", owner, pageName, existingOwner)
		}
		filePath := path.Join(pagesDir, entry.Name())
		content, readErr := fs.ReadFile(source, filePath)
		if readErr != nil {
			return fmt.Errorf("extension %s read page %s: %w", owner, filePath, readErr)
		}
		pageTemplate, cloneErr := extensionBase.Clone()
		if cloneErr != nil {
			return fmt.Errorf("extension %s clone shell for page %s: %w", owner, pageName, cloneErr)
		}
		pageTemplate, parseErr := pageTemplate.Parse(string(content))
		if parseErr != nil {
			return fmt.Errorf("extension %s parse page %s: %w", owner, filePath, parseErr)
		}
		if pageTemplate.Lookup("content") == nil {
			return fmt.Errorf("extension %s page %q must define template %q", owner, pageName, "content")
		}
		pending[pageName] = pageTemplate
	}
	if len(pending) == 0 {
		return fmt.Errorf("extension %s pages directory %s contains no HTML pages", owner, pagesDir)
	}
	for pageName, pageTemplate := range pending {
		r.pages[pageName] = pageTemplate
		r.owners[pageName] = owner
	}
	return nil
}

// RenderPage renders a page wrapped in the shell layout.
// page is the page name without .html (e.g. "home", "contacts").
func (r *Renderer) RenderPage(w http.ResponseWriter, req *http.Request, page string, data interface{}) error {
	return r.renderPage(w, req, page, data, 0)
}

func (r *Renderer) renderPage(w http.ResponseWriter, req *http.Request, page string, data interface{}, status int) error {
	r.mu.RLock()
	tmpl, ok := r.pages[page]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown page template: %q", page)
	}

	shell := r.buildShell(req)
	shell.Body = page
	shell.BodyData = data
	shell.Title = pageTitle(page, shell.Language)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status != 0 {
		w.WriteHeader(status)
	}
	return tmpl.ExecuteTemplate(w, "base", shell)
}

// RenderWithStatus renders a page with a specific HTTP status code.
func (r *Renderer) RenderWithStatus(w http.ResponseWriter, req *http.Request, page string, data interface{}, status int) error {
	return r.renderPage(w, req, page, data, status)
}

// RenderPartial renders a fragment without shell wrapping (for HTMX responses).
func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return r.partials.ExecuteTemplate(w, name, data)
}

// HasPage reports whether the renderer has a parsed template for the given page.
func (r *Renderer) HasPage(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.pages[name]
	return ok
}

// buildShell constructs a ShellViewModel from request context.
func (r *Renderer) buildShell(req *http.Request) *viewmodel.ShellViewModel {
	shell := &viewmodel.ShellViewModel{
		CSRFToken:   CSRFTokenFromRequest(req),
		RequestID:   RequestIDFromContext(req.Context()),
		Version:     r.version,
		Build:       buildinfo.Current(),
		Theme:       "dark",
		Language:    LanguageFromRequest(req),
		CurrentPath: req.URL.Path,
	}

	shell.Layout = "nice"

	if user := UserFromRequest(req); user != nil {
		shell.User = &viewmodel.UserContext{
			ID:       user.ID.String(),
			Username: user.Username,
			Role:     string(user.Role),
			IsAdmin:  user.Role == model.RoleAdmin,
		}
		theme := "dark"
		if user.Theme != nil && *user.Theme != "" {
			theme = *user.Theme
		}
		shell.Theme = theme
		shell.IsDark = theme == "dark"
		if user.Layout != nil && *user.Layout != "" {
			shell.Layout = *user.Layout
		}
	}
	if theme := req.URL.Query().Get("theme"); theme != "" {
		shell.Theme = theme
	}
	if layout := req.URL.Query().Get("layout"); layout != "" {
		shell.Layout = layout
	}
	shell.IsDark = darkTheme(shell.Theme)
	switch shell.Layout {
	case "topbar":
		shell.LayoutCSS = "topbar"
	default:
		shell.LayoutCSS = "sidebar"
	}

	// Resolve extension-contributed nav items from context and flag the one
	// matching the current request path. The root URL ("/") only matches
	// exactly so it isn't flagged active on every page; deeper URLs match by
	// prefix so detail routes (e.g. /contacts/{id}) light up their parent.
	if items := NavItemsFromContext(req.Context()); len(items) > 0 {
		path := req.URL.Path
		resolved := make([]viewmodel.NavItem, len(items))
		activeIndex := -1
		activeLength := -1
		for i, item := range items {
			if navIsActive(item.URL, path) && len(item.URL) > activeLength {
				activeIndex = i
				activeLength = len(item.URL)
			}
		}
		for i, item := range items {
			if key := navTranslationKey(item.URL); key != "" {
				item.Label = TranslateForTemplate(shell.Language, key)
			}
			item.Active = i == activeIndex
			resolved[i] = item
		}
		shell.NavItems = resolved
	}

	return shell
}

// LanguageFromRequest resolves the effective supported language using the
// same preference order as the page renderer: query override, user setting,
// then English.
func LanguageFromRequest(req *http.Request) string {
	language := "en"
	if user := UserFromRequest(req); user != nil && user.Language != nil && i18n.IsSupported(*user.Language) {
		language = *user.Language
	}
	if requested := req.URL.Query().Get("lang"); i18n.IsSupported(requested) {
		language = requested
	}
	return language
}

func navTranslationKey(url string) string {
	switch url {
	case "/":
		return "web.nav.home"
	case "/messages/trash":
		return "web.nav.trash"
	case "/contacts":
		return "web.nav.contacts"
	case "/search":
		return "web.nav.search"
	case "/settings":
		return "web.nav.settings"
	case "/notifications":
		return "web.nav.notifications"
	default:
		return ""
	}
}

func darkTheme(theme string) bool {
	switch theme {
	case "dark", "synthwave", "halloween", "forest", "black", "luxury", "dracula", "business", "night", "coffee", "dim", "sunset":
		return true
	default:
		return false
	}
}

// navIsActive decides whether a nav entry should be highlighted for the given
// request path. The root entry is active only on an exact "/" match.
func navIsActive(navURL, reqPath string) bool {
	if navURL == "/" {
		return reqPath == "/"
	}
	if reqPath == navURL {
		return true
	}
	return strings.HasPrefix(reqPath, navURL+"/")
}

// pageTitle returns a human-readable title for a page name.
func pageTitle(page, language string) string {
	key := ""
	fallback := ""
	switch page {
	case "home":
		key, fallback = "web.nav.home", "Home"
	case "auth_login":
		key, fallback = "web.auth.signin", "Sign In"
	case "auth_change_password":
		key, fallback = "web.password.title", "Change Password"
	case "auth_reset_password":
		key, fallback = "web.reset.title", "Reset Password"
	case "auth_reset_confirm":
		key, fallback = "web.reset.title", "Reset Password"
	case "auth_reset_sent":
		key, fallback = "web.reset.success.title", "Password Reset"
	case "contacts":
		key, fallback = "web.nav.contacts", "Contacts"
	case "contact_detail":
		key, fallback = "web.nav.contacts", "Contacts"
	case "contact_edit":
		key, fallback = "web.contacts.edit", "Edit Contact"
	case "messages":
		key, fallback = "web.home.messages", "Messages"
	case "message_edit":
		key, fallback = "web.messages.edit", "Edit Message"
	case "message_conflict":
		key, fallback = "web.conflict.title", "Sync Conflict"
	case "trash":
		key, fallback = "web.trash.title", "Trash"
	case "search":
		key, fallback = "web.search.title", "Search"
	case "settings":
		key, fallback = "web.settings.title", "Settings"
	case "settings_sessions":
		fallback = "Active Sessions"
	case "notifications":
		key, fallback = "web.notifications.title", "Notifications"
	case "admin_users":
		key, fallback = "web.admin.users.title", "User Administration"
	case "admin_audit":
		key, fallback = "web.admin.audit.title", "Audit Log"
	case "dev_dashboard":
		key, fallback = "web.dev.heading", "Developer Dashboard"
	case "admin_extensions":
		fallback = "Extension Dashboard"
	case "error":
		key, fallback = "web.error.label", "Error"
	default:
		if page == "" {
			return ""
		}
		return strings.ToUpper(page[:1]) + page[1:]
	}
	if key == "" {
		return fallback
	}
	translated := TranslateForTemplate(language, key)
	if translated == key {
		return fallback
	}
	return translated
}
