package handler

import (
	"net/http"
	"net/url"
	"strconv"
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
	ctx.Routes.Protected(http.MethodGet, "/contacts/new", "Contact create form", http.HandlerFunc(h.New))
	ctx.Routes.Protected(http.MethodPost, "/contacts", "Create contact", http.HandlerFunc(h.Create))
	ctx.Routes.Protected(http.MethodGet, "/contacts/{syncId}/edit", "Contact edit form", http.HandlerFunc(h.Edit))
	ctx.Routes.Protected(http.MethodGet, "/contacts/trash/list", "Contact trash list", http.HandlerFunc(h.TrashList))
	ctx.Routes.Protected(http.MethodGet, "/contacts/{syncId}", "Contact detail", http.HandlerFunc(h.Detail))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/update", "Update contact", http.HandlerFunc(h.Update))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/delete", "Delete contact", http.HandlerFunc(h.Delete))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/restore", "Restore contact", http.HandlerFunc(h.Restore))
	return nil
}

func (h *ContactsHandler) List(w http.ResponseWriter, r *http.Request) {
	pageSize := min(max(getIntParam(r, "limit", 12), 1), 50)
	if !r.URL.Query().Has("limit") && r.URL.Query().Has("pageSize") {
		pageSize = min(max(getIntParam(r, "pageSize", 12), 1), 50)
	}
	offset := max(getIntParam(r, "offset", 0), 0)
	if !r.URL.Query().Has("offset") && r.URL.Query().Has("page") {
		offset = (max(getIntParam(r, "page", 1), 1) - 1) * pageSize
	}
	page := offset/pageSize + 1

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
	language := web.LanguageFromRequest(r)
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
			Language:  language,
		}
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  total,
		HasPrevious: page > 1,
		HasNext:     page < totalPages,
		PageSize:    pageSize,
		Language:    web.LanguageFromRequest(r),
	}
	if pagination.HasPrevious {
		pagination.PreviousURL = contactListURL(query, pageSize, max(offset-pageSize, 0))
	}
	if pagination.HasNext {
		pagination.NextURL = contactListURL(query, pageSize, offset+pageSize)
	}

	if err := h.renderer.RenderPage(w, r, "contacts", viewmodel.ContactsPage{
		Contacts:   contactItems,
		Pagination: pagination,
		Query:      query,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ContactsHandler) New(w http.ResponseWriter, r *http.Request) {
	if err := h.renderer.RenderPage(w, r, "contact_edit", viewmodel.ContactForm{
		CSRFToken: web.CSRFTokenFromRequest(r),
		Language:  web.LanguageFromRequest(r),
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ContactsHandler) Edit(w http.ResponseWriter, r *http.Request) {
	contact, err := h.contactService.GetContactBySyncID(r.Context(), chi.URLParam(r, "syncId"))
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if err := h.renderer.RenderPage(w, r, "contact_edit", viewmodel.ContactForm{
		Editing:   true,
		Contact:   storedContactItem(contact),
		CSRFToken: web.CSRFTokenFromRequest(r),
		Language:  web.LanguageFromRequest(r),
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ContactsHandler) TrashList(w http.ResponseWriter, r *http.Request) {
	contacts, _, err := h.contactService.ListDeletedContacts(r.Context(), trashPageLimit, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load deleted contacts")
		return
	}
	items := make([]viewmodel.ContactItem, len(contacts))
	for i := range contacts {
		items[i] = contactSummaryItem(contacts[i], true)
		items[i].CSRFToken = web.CSRFTokenFromRequest(r)
		items[i].Language = web.LanguageFromRequest(r)
	}
	if err := h.renderer.RenderPartial(w, "contact_trash_list", viewmodel.ContactTrashList{
		Contacts: items,
		Language: web.LanguageFromRequest(r),
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

	item := storedContactItem(contact)

	if err := h.renderer.RenderPage(w, r, "contact_detail", viewmodel.ContactDetailPage{
		Contact: item,
		Form: viewmodel.ContactForm{
			Editing: true, Contact: item, CSRFToken: web.CSRFTokenFromRequest(r), Language: web.LanguageFromRequest(r),
		},
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ContactsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	name := r.FormValue("name")
	emails := formList(r, "emails")
	phones := formList(r, "phones")
	socials := formList(r, "socialMedia")
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
		Emails:         formList(r, "emails"),
		Phones:         formList(r, "phones"),
		SocialMedia:    formList(r, "socialMedia"),
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

func (h *ContactsHandler) Restore(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	if err := h.contactService.RestoreContact(r.Context(), syncID); err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/messages/trash", http.StatusSeeOther)
}

func formatEpochMs(ms int64) string {
	if ms == 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	return t.Format("2006-01-02 15:04")
}

func formList(r *http.Request, key string) []string {
	var values []string
	for _, raw := range r.Form[key] {
		for _, value := range strings.Split(raw, ",") {
			if value = strings.TrimSpace(value); value != "" {
				values = append(values, value)
			}
		}
	}
	return values
}

func storedContactItem(contact *model.StoredContact) viewmodel.ContactItem {
	return viewmodel.ContactItem{
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
}

func contactSummaryItem(contact model.ContactSummary, deleted bool) viewmodel.ContactItem {
	return viewmodel.ContactItem{
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
		Deleted:        deleted,
	}
}

func contactListURL(query string, limit, offset int) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	return "/contacts?" + values.Encode()
}
