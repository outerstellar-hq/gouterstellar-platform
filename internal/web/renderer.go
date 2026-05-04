package web

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

type Renderer struct {
	templates *template.Template
}

func NewRenderer(templateFS fs.FS, funcs template.FuncMap) (*Renderer, error) {
	tmpl := template.New("").Funcs(funcs)
	tmpl, err := tmpl.ParseFS(templateFS, "**/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: tmpl}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return r.templates.ExecuteTemplate(w, name, data)
}

func (r *Renderer) RenderWithStatus(w http.ResponseWriter, name string, data interface{}, status int) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return r.templates.ExecuteTemplate(w, name, data)
}

var _ = slog.Default
