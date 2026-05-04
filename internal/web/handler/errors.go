package handler

import (
	"log/slog"
	"net/http"

	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type ErrorHandler struct {
	renderer *web.Renderer
	version  string
}

func NewErrorHandler(renderer *web.Renderer, version string) *ErrorHandler {
	return &ErrorHandler{
		renderer: renderer,
		version:  version,
	}
}

func (h *ErrorHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	h.renderer.RenderWithStatus(w, "error.html", viewmodel.ErrorPage{
		StatusCode: http.StatusNotFound,
		Title:      "Not Found",
		Message:    "The page you are looking for does not exist.",
		RequestID:  web.RequestIDFromContext(r.Context()),
	}, http.StatusNotFound)
}

func (h *ErrorHandler) InternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("Internal server error",
		"error", err,
		"path", r.URL.Path,
		"method", r.Method,
	)

	h.renderer.RenderWithStatus(w, "error.html", viewmodel.ErrorPage{
		StatusCode: http.StatusInternalServerError,
		Title:      "Internal Server Error",
		Message:    "Something went wrong. Please try again later.",
		RequestID:  web.RequestIDFromContext(r.Context()),
	}, http.StatusInternalServerError)
}
