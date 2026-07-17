package handler

import (
	"net/http"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

const trashPageLimit int32 = 100

type TrashHandler struct {
	messageService *service.MessageService
	contactService *service.ContactService
	renderer       *web.Renderer
}

func NewTrashHandler(messageService *service.MessageService, contactService *service.ContactService, renderer *web.Renderer) *TrashHandler {
	return &TrashHandler{messageService: messageService, contactService: contactService, renderer: renderer}
}

func (h *TrashHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/messages/trash", "Trash", http.HandlerFunc(h.Show))
	return nil
}

func (h *TrashHandler) Show(w http.ResponseWriter, r *http.Request) {
	pageSize := min(max(getIntParam(r, "limit", int(trashPageLimit)), 1), int(trashPageLimit))
	offset := max(getIntParam(r, "offset", 0), 0)
	messages, err := h.messageService.ListDeletedMessages(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		h.renderLoadError(w, r)
		return
	}

	contacts, contactTotal, err := h.contactService.ListDeletedContacts(r.Context(), trashPageLimit, 0)
	if err != nil {
		h.renderLoadError(w, r)
		return
	}

	csrfToken := web.CSRFTokenFromRequest(r)
	language := web.LanguageFromRequest(r)
	messageItems := make([]viewmodel.MessageItem, len(messages.Items))
	for i, message := range messages.Items {
		messageItems[i] = viewmodel.MessageItem{
			SyncID:       message.SyncID,
			Author:       message.Author,
			Content:      message.Content,
			UpdatedAt:    message.UpdatedAtLabel(),
			UpdatedLabel: message.UpdatedAtLabel(),
			Dirty:        message.Dirty,
			Version:      message.Version,
			HasConflict:  message.HasConflict,
			Deleted:      true,
			CSRFToken:    csrfToken,
			Language:     language,
		}
	}

	contactItems := make([]viewmodel.ContactItem, len(contacts))
	for i, contact := range contacts {
		contactItems[i] = contactSummaryItem(contact, true)
		contactItems[i].CSRFToken = csrfToken
		contactItems[i].Language = language
	}

	messagePagination := viewmodel.PaginationInfo{
		CurrentPage: messages.Metadata.CurrentPage,
		TotalPages:  messages.Metadata.TotalPages,
		TotalItems:  messages.Metadata.TotalItems,
		HasPrevious: messages.Metadata.HasPrevious,
		HasNext:     messages.Metadata.HasNext,
		PageSize:    messages.Metadata.PageSize,
		Language:    language,
	}
	if messagePagination.HasPrevious {
		messagePagination.PreviousURL = messagePageURL("/messages/trash", "", 0, pageSize, max(offset-pageSize, 0))
	}
	if messagePagination.HasNext {
		messagePagination.NextURL = messagePageURL("/messages/trash", "", 0, pageSize, offset+pageSize)
	}

	if err := h.renderer.RenderPage(w, r, "trash", viewmodel.TrashPage{
		MessageList: viewmodel.MessagesPage{
			Messages:   messageItems,
			Pagination: messagePagination,
			RefreshURL: messageComponentURL("", 0, pageSize, offset, language, true),
			Trash:      true,
		},
		ContactList: viewmodel.ContactTrashList{
			Contacts:   contactItems,
			Language:   language,
			RefreshURL: "/contacts/trash/list?lang=" + language,
		},
		MessageTotal: messages.Metadata.TotalItems,
		ContactTotal: contactTotal,
		DeletedTotal: messages.Metadata.TotalItems + contactTotal,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *TrashHandler) renderLoadError(w http.ResponseWriter, r *http.Request) {
	_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
		StatusCode: http.StatusInternalServerError,
		Title:      "Error",
		Message:    "Failed to load deleted items",
	}, http.StatusInternalServerError)
}
