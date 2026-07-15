package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

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

// ContributeRoutes registers the contacts UI routes (protected).
func (h *ContactsHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/contacts", "Contacts list", http.HandlerFunc(h.List))
	ctx.Routes.Protected(http.MethodGet, "/contacts/{syncId}", "Contact detail", http.HandlerFunc(h.Detail))
	ctx.Routes.Protected(http.MethodPost, "/contacts/create", "Create contact", http.HandlerFunc(h.Create))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/update", "Update contact", http.HandlerFunc(h.Update))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/delete", "Delete contact", http.HandlerFunc(h.Delete))
	return nil
}

func (h *ContactsHandler) List(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// SearchContacts returns (items, total, err) and scopes both the rows and
	// the count to the query. When there is no query we fall back to the plain
	// list + count so the unfiltered page stays cache-friendly in the repo.
	var contacts []model.ContactSummary
	var total int64
	var err error
	if query != "" {
		contacts, total, err = h.contactService.SearchContacts(r.Context(), query, safeInt32(pageSize), safeInt32(offset))
	} else {
		contacts, err = h.contactService.ListContacts(r.Context(), safeInt32(pageSize), safeInt32(offset))
		if err == nil {
			total, _ = h.contactService.CountContacts(r.Context())
		}
	}
	if err != nil {
		_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError,
			Title:      "Error",
			Message:    "Failed to load contacts",
		}, http.StatusInternalServerError)
		return
	}

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

	if err := h.renderer.RenderPage(w, r, "contacts", viewmodel.ContactsPage{
		Contacts:   contactItems,
		Pagination: pagination,
		Query:      query,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ContactsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	contact, err := h.contactService.GetContactBySyncID(r.Context(), syncID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	item := viewmodel.ContactItem{
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
		Deleted:        contact.Deleted,
	}

	if err := h.renderer.RenderPage(w, r, "contact_detail", viewmodel.ContactDetailPage{Contact: item}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
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
		SyncID:         syncID,
		Name:           r.FormValue("name"),
		Emails:         r.Form["emails"],
		Phones:         r.Form["phones"],
		SocialMedia:    r.Form["socials"],
		Company:        r.FormValue("company"),
		CompanyAddress: r.FormValue("companyAddress"),
		Department:     r.FormValue("department"),
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
