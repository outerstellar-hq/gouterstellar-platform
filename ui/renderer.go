// Package ui provides a shared, application-neutral HTML shell for server-rendered Go applications.
package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"strings"
)

//go:embed templates/shell.html
var shellTemplates embed.FS

// Options declares the application-owned templates composed into the shared shell.
type Options struct {
	Templates fs.FS
	Patterns  []string
	Functions template.FuncMap
}

// Renderer is an immutable parsed template set safe for concurrent rendering.
type Renderer struct {
	templates *template.Template
}

// NewRenderer parses the consumer's content templates and then installs the
// shared shell as the authoritative owner of all shared template names.
// Consumer templates must define application-content; all product-specific HTML
// remains in the consumer repository.
func NewRenderer(opts Options) (*Renderer, error) {
	if opts.Templates == nil {
		return nil, fmt.Errorf("application template filesystem is required")
	}
	if len(opts.Patterns) == 0 {
		return nil, fmt.Errorf("at least one application template pattern is required")
	}
	parsed, err := template.New("shared-ui").Funcs(opts.Functions).ParseFS(opts.Templates, opts.Patterns...)
	if err != nil {
		return nil, fmt.Errorf("parse application templates: %w", err)
	}
	if parsed.Lookup("application-content") == nil {
		return nil, fmt.Errorf("application templates must define %q", "application-content")
	}
	parsed, err = parsed.ParseFS(shellTemplates, "templates/shell.html")
	if err != nil {
		return nil, fmt.Errorf("parse shared UI shell: %w", err)
	}
	return &Renderer{templates: parsed}, nil
}

// Render executes application data inside the shared shell.
func (r *Renderer) Render(w io.Writer, shell Shell, data any) error {
	if r == nil || r.templates == nil {
		return fmt.Errorf("UI renderer is not initialized")
	}
	if err := shell.Validate(); err != nil {
		return err
	}
	if err := r.templates.ExecuteTemplate(w, "shared-shell", Page{Shell: shell, Data: data}); err != nil {
		return fmt.Errorf("render shared UI shell: %w", err)
	}
	return nil
}

// ExecuteTemplate renders an application-owned named template without the shell.
func (r *Renderer) ExecuteTemplate(w io.Writer, name string, data any) error {
	if r == nil || r.templates == nil {
		return fmt.Errorf("UI renderer is not initialized")
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("template name is required")
	}
	if r.templates.Lookup(name) == nil {
		return fmt.Errorf("unknown application template %q", name)
	}
	return r.templates.ExecuteTemplate(w, name, data)
}

// Page is the shared shell execution model. Data is passed unchanged to the consumer template.
type Page struct {
	Shell Shell
	Data  any
}
