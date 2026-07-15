package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type Renderer struct {
	pages   map[string]*template.Template
	version string
}

func NewRenderer(templateFS fs.FS, funcs template.FuncMap, version string) (*Renderer, error) {
	partials, err := fs.Glob(templateFS, "template/partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("find template partials: %w", err)
	}
	pageFiles, err := fs.Glob(templateFS, "template/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("find page templates: %w", err)
	}

	pages := make(map[string]*template.Template, len(pageFiles))
	for _, pageFile := range pageFiles {
		files := append([]string{"template/base.html"}, partials...)
		files = append(files, pageFile)
		parsed, parseErr := template.New(filepath.Base(pageFile)).Funcs(funcs).ParseFS(templateFS, files...)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", pageFile, parseErr)
		}
		pages[filepath.Base(pageFile)] = parsed
	}
	return &Renderer{pages: pages, version: version}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, request *http.Request, name string, data interface{}) error {
	return r.render(w, request, name, data, http.StatusOK)
}

func (r *Renderer) RenderWithStatus(w http.ResponseWriter, request *http.Request, name string, data interface{}, status int) error {
	return r.render(w, request, name, data, status)
}

func (r *Renderer) render(w http.ResponseWriter, request *http.Request, name string, data interface{}, status int) error {
	isFragment := strings.HasPrefix(filepath.ToSlash(name), "components/")
	name = pageAlias(name)
	page, ok := r.pages[name]
	if !ok {
		return fmt.Errorf("template %q does not exist", name)
	}

	view := data
	templateName := "content"
	if !isFragment {
		view = r.shell(request, name, data)
		templateName = "base"
	}

	var content bytes.Buffer
	if err := page.ExecuteTemplate(&content, templateName, view); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(content.Bytes())
	return err
}

func (r *Renderer) shell(request *http.Request, name string, data interface{}) viewmodel.ShellViewModel {
	theme := "light"
	language := "en"
	var userContext *viewmodel.UserContext
	if user := UserFromRequest(request); user != nil {
		if user.Theme != nil && *user.Theme != "" {
			theme = *user.Theme
		}
		if user.Language != nil && *user.Language != "" {
			language = *user.Language
		}
		userContext = &viewmodel.UserContext{
			ID: user.ID.String(), Username: user.Username, Role: string(user.Role), IsAdmin: user.Role == model.RoleAdmin,
		}
	}
	page := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if page == "search" {
		page = "messages"
	}
	return viewmodel.ShellViewModel{
		Title: titleForPage(page), User: userContext, Theme: theme, IsDark: theme == "dark", Language: language,
		CSRFToken: CSRFTokenFromRequest(request), Version: r.version, RequestID: RequestIDFromContext(request.Context()),
		Body: page, BodyData: data,
	}
}

func titleForPage(page string) string {
	titles := map[string]string{
		"home": "Home", "messages": "Messages", "contacts": "Contacts", "notifications": "Notifications",
		"settings": "Settings", "trash": "Trash", "admin_users": "User Management", "admin_audit": "Audit Log",
		"auth_login": "Sign in", "auth_change_password": "Change password", "auth_reset_password": "Reset password",
		"auth_reset_sent": "Password reset", "dev_dashboard": "Developer Dashboard", "error": "Error",
	}
	if title, ok := titles[page]; ok {
		return title
	}
	return "Outerstellar Platform"
}

func pageAlias(name string) string {
	base := filepath.Base(name)
	if base == "search.html" {
		return "messages.html"
	}
	return base
}
