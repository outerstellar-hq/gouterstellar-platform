package handler

import (
	"net/http"
	"strings"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type MessagesHandler struct {
	messageService *service.MessageService
	renderer       *web.Renderer
}

func NewMessagesHandler(msgSvc *service.MessageService, renderer *web.Renderer) *MessagesHandler {
	return &MessagesHandler{
		messageService: msgSvc,
		renderer:       renderer,
	}
}

func (h *MessagesHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	year := getIntParam(r, "year", 0)

	// Text search takes precedence over the year filter: a search query scopes
	// results to matching text, and applying a year on top would hide matches
	// from other years the user is searching across.
	var result *model.PagedResult[model.MessageSummary]
	var err error
	switch {
	case query != "":
		result, err = h.messageService.SearchMessages(r.Context(), query, safeInt32(pageSize), safeInt32(offset))
	case year > 0:
		result, err = h.messageService.ListMessagesByYear(r.Context(), year, safeInt32(pageSize), safeInt32(offset))
	default:
		result, err = h.messageService.ListMessages(r.Context(), safeInt32(pageSize), safeInt32(offset))
	}
	if err != nil {
		_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError,
			Title:      "Error",
			Message:    "Failed to load messages",
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

	// The year filter is always populated so the UI is consistent whether or not
	// a query/term is active. Errors here are non-fatal: an empty year list just
	// hides the filter.
	years, yearsErr := h.messageService.GetMessageYears(r.Context())
	if yearsErr != nil {
		years = nil
	}

	if err := h.renderer.RenderPage(w, r, "messages", viewmodel.MessagesPage{
		Messages:   messageItems,
		Pagination: pagination,
		Query:      query,
		Year:       year,
		Years:      years,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
