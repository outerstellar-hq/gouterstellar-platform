package handler

import (
	"net/http"

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

	result, err := h.messageService.ListMessages(r.Context(), safeInt32(pageSize), safeInt32(offset))
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

	if err := h.renderer.RenderPage(w, r, "messages", viewmodel.MessagesPage{
		Messages:   messageItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
