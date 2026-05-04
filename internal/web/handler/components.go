package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type ComponentsHandler struct {
	messageService *service.MessageService
	contactService *service.ContactService
	renderer       *web.Renderer
}

func NewComponentsHandler(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	renderer *web.Renderer,
) *ComponentsHandler {
	return &ComponentsHandler{
		messageService: msgSvc,
		contactService: contactSvc,
		renderer:       renderer,
	}
}

func (h *ComponentsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/components/message-list", h.MessageList)
	r.Get("/components/contact-list", h.ContactList)
}

func (h *ComponentsHandler) MessageList(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	result, err := h.messageService.ListMessages(r.Context(), int32(pageSize), int32(offset))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load messages")
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

	if err := h.renderer.Render(w, "components/message_list.html", viewmodel.MessagesPage{
		Messages:   messageItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ComponentsHandler) ContactList(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	contacts, err := h.contactService.ListContacts(r.Context(), int32(pageSize), int32(offset))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load contacts")
		return
	}

	total, _ := h.contactService.CountContacts(r.Context())
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	contactItems := make([]viewmodel.ContactItem, len(contacts))
	for i, c := range contacts {
		contactItems[i] = viewmodel.ContactItem{
			SyncID:    c.SyncID,
			Name:      c.Name,
			Emails:    c.Emails,
			Phones:    c.Phones,
			Social:    c.SocialMedia,
			Company:   c.Company,
			UpdatedAt: formatEpochMs(c.UpdatedAtEpochMs),
			Dirty:     c.Dirty,
		}
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  total,
		HasPrevious: page > 1,
		HasNext:     page < totalPages,
		PageSize:    pageSize,
	}

	if err := h.renderer.Render(w, "components/contact_list.html", viewmodel.ContactsPage{
		Contacts:   contactItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
