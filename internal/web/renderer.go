package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

// Renderer renders HTML pages through a shared layout (base.html) and
// partials for HTMX fragment responses. Each page is pre-cloned from the
// base template set so its {{ define "content" }} block resolves cleanly.
type Renderer struct {
	pages    map[string]*template.Template
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

	return &Renderer{pages: pages, partials: partials, version: version}, nil
}

// RenderPage renders a page wrapped in the shell layout.
// page is the page name without .html (e.g. "home", "contacts").
func (r *Renderer) RenderPage(w http.ResponseWriter, req *http.Request, page string, data interface{}) error {
	tmpl, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("unknown page template: %q", page)
	}

	shell := r.buildShell(req)
	shell.Body = page
	shell.BodyData = data
	shell.Title = pageTitle(page)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "base", shell)
}

// RenderWithStatus renders a page with a specific HTTP status code.
func (r *Renderer) RenderWithStatus(w http.ResponseWriter, req *http.Request, page string, data interface{}, status int) error {
	w.WriteHeader(status)
	return r.RenderPage(w, req, page, data)
}

// RenderPartial renders a fragment without shell wrapping (for HTMX responses).
func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return r.partials.ExecuteTemplate(w, name, data)
}

// HasPage reports whether the renderer has a parsed template for the given page.
func (r *Renderer) HasPage(name string) bool {
	_, ok := r.pages[name]
	return ok
}

// buildShell constructs a ShellViewModel from request context.
func (r *Renderer) buildShell(req *http.Request) *viewmodel.ShellViewModel {
	shell := &viewmodel.ShellViewModel{
		CSRFToken: CSRFTokenFromRequest(req),
		RequestID: RequestIDFromContext(req.Context()),
		Version:   r.version,
		Theme:     "light",
	}

	if user := UserFromRequest(req); user != nil {
		shell.User = &viewmodel.UserContext{
			ID:       user.ID.String(),
			Username: user.Username,
			Role:     string(user.Role),
			IsAdmin:  user.Role == model.RoleAdmin,
		}
		theme := "light"
		if user.Theme != nil && *user.Theme != "" {
			theme = *user.Theme
		}
		shell.Theme = theme
		shell.IsDark = theme == "dark"
		if user.Language != nil {
			shell.Language = *user.Language
		}
	}

	// Resolve extension-contributed nav items from context and flag the one
	// matching the current request path. The root URL ("/") only matches
	// exactly so it isn't flagged active on every page; deeper URLs match by
	// prefix so detail routes (e.g. /contacts/{id}) light up their parent.
	if items := NavItemsFromContext(req.Context()); len(items) > 0 {
		path := req.URL.Path
		resolved := make([]viewmodel.NavItem, len(items))
		for i, item := range items {
			item.Active = navIsActive(item.URL, path)
			resolved[i] = item
		}
		shell.NavItems = resolved
	}

	return shell
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
func pageTitle(page string) string {
	switch page {
	case "home":
		return "Dashboard"
	case "auth_login":
		return "Sign In"
	case "auth_change_password":
		return "Change Password"
	case "auth_reset_password":
		return "Reset Password"
	case "auth_reset_confirm":
		return "Set New Password"
	case "auth_reset_sent":
		return "Reset Sent"
	case "contacts":
		return "Contacts"
	case "contact_detail":
		return "Contact"
	case "messages":
		return "Messages"
	case "search":
		return "Search"
	case "settings":
		return "Settings"
	case "settings_sessions":
		return "Active Sessions"
	case "notifications":
		return "Notifications"
	case "admin_users":
		return "User Management"
	case "admin_audit":
		return "Audit Log"
	case "dev_dashboard":
		return "Dev Dashboard"
	case "error":
		return "Error"
	default:
		return strings.Title(page)
	}
}
