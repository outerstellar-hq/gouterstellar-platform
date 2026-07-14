package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type SearchHandler struct {
	messageService *service.MessageService
	contactService *service.ContactService
	renderer       *web.Renderer
}

func NewSearchHandler(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	renderer *web.Renderer,
) *SearchHandler {
	return &SearchHandler{
		messageService: msgSvc,
		contactService: contactSvc,
		renderer:       renderer,
	}
}

func (h *SearchHandler) RegisterRoutes(r chi.Router) {
	r.Get("/search", h.Search)
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		if err := h.renderer.RenderPage(w, r, "search", viewmodel.MessagesPage{
			Query: query,
		}); err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
		return
	}

	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	result, err := h.messageService.SearchMessages(r.Context(), query, safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError,
			Title:      "Error",
			Message:    "Search failed",
		}, http.StatusInternalServerError)
		return
	}

	messageItems := make([]viewmodel.MessageItem, len(result.Items))
	for i, m := range result.Items {
		messageItems[i] = viewmodel.MessageItem{
			SyncID:       m.SyncID,
			Author:       m.Author,
			Content:      m.Content,
			UpdatedAt:    m.UpdatedAtLabel(),
			UpdatedLabel: m.UpdatedAtLabel(),
			Dirty:        m.Dirty,
			Version:      m.Version,
			HasConflict:  m.HasConflict,
		}
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: result.Metadata.CurrentPage,
		TotalPages:  result.Metadata.TotalPages,
		TotalItems:  result.Metadata.TotalItems,
		HasPrevious: result.Metadata.HasPrevious,
		HasNext:     result.Metadata.HasNext,
		PageSize:    result.Metadata.PageSize,
	}

	if err := h.renderer.RenderPage(w, r, "search", viewmodel.MessagesPage{
		Messages:   messageItems,
		Pagination: pagination,
		Query:      query,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
