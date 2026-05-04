package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type ContactsHandler struct {
	contactService *service.ContactService
	renderer       *web.Renderer
}

func NewContactsHandler(contactSvc *service.ContactService, renderer *web.Renderer) *ContactsHandler {
	return &ContactsHandler{
		contactService: contactSvc,
		renderer:       renderer,
	}
}

func (h *ContactsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/contacts", h.List)
	r.Get("/contacts/{syncId}", h.Detail)
	r.Post("/contacts/create", h.Create)
	r.Post("/contacts/{syncId}/update", h.Update)
	r.Post("/contacts/{syncId}/delete", h.Delete)
}

func (h *ContactsHandler) List(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	contacts, err := h.contactService.ListContacts(r.Context(), int32(pageSize), int32(offset))
	if err != nil {
		h.renderer.RenderWithStatus(w, "error.html", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError,
			Title:      "Error",
			Message:    "Failed to load contacts",
		}, http.StatusInternalServerError)
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
			Deleted:   false,
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

	h.renderer.Render(w, "contacts.html", viewmodel.ContactsPage{
		Contacts:   contactItems,
		Pagination: pagination,
	})
}

func (h *ContactsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	contact, err := h.contactService.GetContactBySyncID(r.Context(), syncID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, contact)
}

func (h *ContactsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	name := r.FormValue("name")
	emails := r.Form["emails"]
	phones := r.Form["phones"]
	socials := r.Form["socials"]
	company := r.FormValue("company")
	companyAddress := r.FormValue("companyAddress")
	department := r.FormValue("department")

	_, err := h.contactService.CreateContact(r.Context(), name, emails, phones, socials, company, companyAddress, department)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

func (h *ContactsHandler) Update(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	contact := &model.StoredContact{
		SyncID:           syncID,
		Name:             r.FormValue("name"),
		Emails:           r.Form["emails"],
		Phones:           r.Form["phones"],
		SocialMedia:      r.Form["socials"],
		Company:          r.FormValue("company"),
		CompanyAddress:   r.FormValue("companyAddress"),
		Department:       r.FormValue("department"),
	}

	_, err := h.contactService.UpdateContact(r.Context(), contact)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

func (h *ContactsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	err := h.contactService.DeleteContact(r.Context(), syncID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

func formatEpochMs(ms int64) string {
	if ms == 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	return t.Format("2006-01-02 15:04")
}
