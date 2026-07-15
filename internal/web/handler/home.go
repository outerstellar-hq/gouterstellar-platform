package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type HomeHandler struct {
	messageService  *service.MessageService
	contactService  *service.ContactService
	securityService *service.SecurityService
	renderer        *web.Renderer
	version         string
}

func NewHomeHandler(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	secSvc *service.SecurityService,
	renderer *web.Renderer,
	version string,
) *HomeHandler {
	return &HomeHandler{
		messageService:  msgSvc,
		contactService:  contactSvc,
		securityService: secSvc,
		renderer:        renderer,
		version:         version,
	}
}

func (h *HomeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.Show)
	r.Get("/messages", h.Messages)
	r.Post("/messages", h.CreateMessage)
	r.Post("/messages/{syncId}/delete", h.DeleteMessage)
	r.Post("/messages/restore/{syncId}", h.RestoreMessage)
	r.Get("/messages/trash", h.Trash)
}

func (h *HomeHandler) Show(w http.ResponseWriter, r *http.Request) {
	messageCount, _ := h.countMessages(r)
	contactCount, _ := h.contactService.CountContacts(r.Context())
	userCount, _ := h.securityService.CountUsers(r.Context())

	page := viewmodel.HomePage{
		MessageCount: messageCount,
		ContactCount: contactCount,
		UserCount:    userCount,
	}

	if err := h.renderer.Render(w, r, "home.html", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *HomeHandler) Messages(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	result, err := h.messageService.ListMessages(r.Context(), safeInt32(pageSize), safeInt32((page-1)*pageSize))
	if err != nil {
		_ = h.renderer.RenderWithStatus(w, r, "error.html", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError, Title: "Error", Message: "Failed to load messages",
		}, http.StatusInternalServerError)
		return
	}
	items := make([]viewmodel.MessageItem, len(result.Items))
	for i, message := range result.Items {
		items[i] = messageItem(message)
	}
	metadata := result.Metadata
	if err := h.renderer.Render(w, r, "messages.html", viewmodel.MessagesPage{
		Messages: items,
		Pagination: viewmodel.PaginationInfo{
			CurrentPage: metadata.CurrentPage, TotalPages: metadata.TotalPages, TotalItems: metadata.TotalItems,
			HasPrevious: metadata.HasPrevious, HasNext: metadata.HasNext, PageSize: metadata.PageSize,
		},
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func messageItem(message model.MessageSummary) viewmodel.MessageItem {
	return viewmodel.MessageItem{
		SyncID: message.SyncID, Author: message.Author, Content: message.Content,
		UpdatedAt: message.UpdatedAtLabel(), UpdatedLabel: message.UpdatedAtLabel(), Dirty: message.Dirty,
		Version: message.Version, HasConflict: message.HasConflict,
	}
}

func (h *HomeHandler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}
	if _, err := h.messageService.CreateServerMessage(r.Context(), r.FormValue("author"), r.FormValue("content")); err != nil {
		handleServiceError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *HomeHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	if err := h.messageService.DeleteMessage(r.Context(), chi.URLParam(r, "syncId")); err != nil {
		handleServiceError(w, err)
		return
	}
	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

func (h *HomeHandler) RestoreMessage(w http.ResponseWriter, r *http.Request) {
	if err := h.messageService.Restore(r.Context(), chi.URLParam(r, "syncId")); err != nil {
		handleServiceError(w, err)
		return
	}
	http.Redirect(w, r, "/messages/trash", http.StatusFound)
}

func (h *HomeHandler) Trash(w http.ResponseWriter, r *http.Request) {
	messages, err := h.messageService.ListDeletedMessages(r.Context(), 100, 0)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	contacts, err := h.contactService.ListDeletedContacts(r.Context(), 100, 0)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	messageItems := make([]viewmodel.MessageItem, len(messages.Items))
	for i, message := range messages.Items {
		messageItems[i] = viewmodel.MessageItem{SyncID: message.SyncID, Author: message.Author, Content: message.Content, Deleted: true}
	}
	contactItems := make([]viewmodel.ContactItem, len(contacts))
	for i, contact := range contacts {
		contactItems[i] = viewmodel.ContactItem{SyncID: contact.SyncID, Name: contact.Name, Emails: contact.Emails, Phones: contact.Phones, Company: contact.Company, Deleted: true}
	}
	if err := h.renderer.Render(w, r, "trash.html", viewmodel.TrashPage{Messages: messageItems, Contacts: contactItems}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *HomeHandler) countMessages(r *http.Request) (int64, error) {
	result, err := h.messageService.ListMessages(r.Context(), 1, 0)
	if err != nil {
		return 0, err
	}
	return result.Metadata.TotalItems, nil
}
