package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

type ErrorHandler struct {
	renderer *web.Renderer
	version  string
}

func (h *ErrorHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Public(http.MethodGet, "/errors/{kind}", "Themed error page", http.HandlerFunc(h.Show))
	ctx.Routes.Public(http.MethodGet, "/errors/components/help/{kind}", "Error help component", http.HandlerFunc(h.Help))
	return nil
}

func (h *ErrorHandler) Show(w http.ResponseWriter, r *http.Request) {
	page, ok := errorPage(chi.URLParam(r, "kind"), web.LanguageFromRequest(r))
	if !ok {
		writeError(w, http.StatusBadRequest, "Unknown error page kind")
		return
	}
	if err := h.renderer.RenderPage(w, r, "error", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ErrorHandler) Help(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	language := web.LanguageFromRequest(r)
	_, ok := errorPage(kind, language)
	if !ok {
		writeError(w, http.StatusBadRequest, "Unknown error help kind")
		return
	}
	if err := h.renderer.RenderPartial(w, "error_help", viewmodel.ErrorHelp{
		Title: web.TranslateForTemplate(language, "web.error."+kind+".help.title"),
		Items: []string{
			web.TranslateForTemplate(language, "web.error."+kind+".help.item1"),
			web.TranslateForTemplate(language, "web.error."+kind+".help.item2"),
			web.TranslateForTemplate(language, "web.error."+kind+".help.item3"),
		},
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func errorPage(kind, language string) (viewmodel.ErrorPage, bool) {
	switch kind {
	case "not-found":
		return viewmodel.ErrorPage{StatusCode: http.StatusNotFound, Title: web.TranslateForTemplate(language, "web.error.not-found.title"), Message: web.TranslateForTemplate(language, "web.error.not-found.message")}, true
	case "server-error":
		return viewmodel.ErrorPage{StatusCode: http.StatusInternalServerError, Title: web.TranslateForTemplate(language, "web.error.server-error.title"), Message: web.TranslateForTemplate(language, "web.error.server-error.message")}, true
	case "forbidden":
		return viewmodel.ErrorPage{StatusCode: http.StatusForbidden, Title: web.TranslateForTemplate(language, "web.error.forbidden.title"), Message: web.TranslateForTemplate(language, "web.error.forbidden.message")}, true
	default:
		return viewmodel.ErrorPage{}, false
	}
}

func NewErrorHandler(renderer *web.Renderer, version string) *ErrorHandler {
	return &ErrorHandler{
		renderer: renderer,
		version:  version,
	}
}

func (h *ErrorHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	requestID := web.RequestIDFromContext(r.Context())
	if isAPIPath(r.URL.Path) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":     "not_found",
			"message":   "The requested API resource does not exist.",
			"requestId": requestID,
		})
		return
	}

	page, _ := errorPage("not-found", web.LanguageFromRequest(r))
	page.RequestID = requestID
	if err := h.renderer.RenderWithStatus(w, r, "error", page, http.StatusNotFound); err != nil {
		slog.Error("render not found page", "error", err, "requestId", requestID)
		http.Error(w, "Page not found\n\nThe requested page does not exist.\nReference: "+requestID, http.StatusNotFound)
	}
}

func (h *ErrorHandler) InternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error(
		"Internal server error",
		"error", err,
		"path", r.URL.Path,
		"method", r.Method,
	)

	requestID := web.RequestIDFromContext(r.Context())
	if isAPIPath(r.URL.Path) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":     "internal_server_error",
			"message":   "The request could not be completed.",
			"requestId": requestID,
		})
		return
	}
	if strings.EqualFold(r.Header.Get("HX-Request"), "true") {
		writeText(w, http.StatusInternalServerError, "Action failed")
		return
	}

	page, _ := errorPage("server-error", web.LanguageFromRequest(r))
	page.RequestID = requestID
	if renderErr := h.renderer.RenderWithStatus(w, r, "error", page, http.StatusInternalServerError); renderErr != nil {
		slog.Error("render internal error page", "error", renderErr, "requestId", requestID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}
