package handler

import (
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
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
	messages, err := h.messageService.ListDeletedMessages(r.Context(), trashPageLimit, 0)
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
		}
	}

	contactItems := make([]viewmodel.ContactItem, len(contacts))
	for i, contact := range contacts {
		contactItems[i] = viewmodel.ContactItem{
			SyncID:         contact.SyncID,
			Name:           contact.Name,
			Emails:         contact.Emails,
			Phones:         contact.Phones,
			Social:         contact.SocialMedia,
			Company:        contact.Company,
			CompanyAddress: contact.CompanyAddress,
			Department:     contact.Department,
			UpdatedAt:      formatEpochMs(contact.UpdatedAtEpochMs),
			Dirty:          contact.Dirty,
			Deleted:        true,
		}
	}

	if err := h.renderer.RenderPage(w, r, "trash", viewmodel.TrashPage{
		Messages:     messageItems,
		Contacts:     contactItems,
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
