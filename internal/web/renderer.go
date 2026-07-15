package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

type Renderer struct {
	pages map[string]*template.Template
}

func NewRenderer(templateFS fs.FS, funcs template.FuncMap) (*Renderer, error) {
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
		files := append(append([]string{}, partials...), pageFile)
		parsed, parseErr := template.New(filepath.Base(pageFile)).Funcs(funcs).ParseFS(templateFS, files...)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", pageFile, parseErr)
		}
		pages[filepath.Base(pageFile)] = parsed
	}
	return &Renderer{pages: pages}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	return r.render(w, name, data, http.StatusOK)
}

func (r *Renderer) RenderWithStatus(w http.ResponseWriter, name string, data interface{}, status int) error {
	return r.render(w, name, data, status)
}

func (r *Renderer) render(w http.ResponseWriter, name string, data interface{}, status int) error {
	isFragment := strings.HasPrefix(filepath.ToSlash(name), "components/")
	name = pageAlias(name)
	page, ok := r.pages[name]
	if !ok {
		return fmt.Errorf("template %q does not exist", name)
	}

	var content bytes.Buffer
	if err := page.ExecuteTemplate(&content, "content", data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if isFragment {
		_, err := w.Write(content.Bytes())
		return err
	}
	_, err := fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Outerstellar Platform</title><link rel="stylesheet" href="/static/css/main.css"><script src="/static/js/platform.js" defer></script></head><body><main>%s</main></body></html>`, content.String())
	return err
}

func pageAlias(name string) string {
	base := filepath.Base(name)
	if base == "search.html" {
		return "messages.html"
	}
	return base
}
