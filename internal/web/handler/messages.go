package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

type MessagesHandler struct {
	messageService *service.MessageService
	voteService    messageVoteService
	renderer       *web.Renderer
}

func NewMessagesHandler(msgSvc *service.MessageService, voteSvc messageVoteService, renderer *web.Renderer) *MessagesHandler {
	return &MessagesHandler{
		messageService: msgSvc,
		voteService:    voteSvc,
		renderer:       renderer,
	}
}

// ContributeRoutes registers the messages UI routes (protected).
func (h *MessagesHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/", "Home message workspace", http.HandlerFunc(h.Show))
	ctx.Routes.Protected(http.MethodGet, "/messages", "Messages", http.HandlerFunc(h.Show))
	ctx.Routes.Protected(http.MethodPost, "/messages", "Create message", http.HandlerFunc(h.Create))
	ctx.Routes.Protected(http.MethodGet, "/messages/{syncId}/edit", "Edit message", http.HandlerFunc(h.Edit))
	ctx.Routes.Protected(http.MethodPost, "/messages/{syncId}/update", "Update message", http.HandlerFunc(h.Update))
	ctx.Routes.Protected(http.MethodPost, "/messages/{syncId}/delete", "Delete message", http.HandlerFunc(h.Delete))
	ctx.Routes.Protected(http.MethodPost, "/messages/restore/{syncId}", "Restore message", http.HandlerFunc(h.Restore))
	ctx.Routes.Protected(http.MethodGet, "/messages/resolve/{syncId}", "Show conflict resolution", http.HandlerFunc(h.Resolve))
	ctx.Routes.Protected(http.MethodPost, "/messages/resolve/{syncId}", "Resolve conflict", http.HandlerFunc(h.ResolveConflict))
	return nil
}

func (h *MessagesHandler) Edit(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")
	message, err := h.messageService.FindBySyncID(r.Context(), syncID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if message.Deleted {
		handleServiceError(w, &model.MessageNotFoundError{SyncID: syncID})
		return
	}

	h.renderEdit(w, r, viewmodel.MessageEditPage{SyncID: syncID, Author: message.Author, Content: message.Content}, http.StatusOK)
}

func (h *MessagesHandler) Update(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	syncID := chi.URLParam(r, "syncId")
	author := r.FormValue("author")
	content := r.FormValue("content")
	if _, err := h.messageService.UpdateMessage(r.Context(), syncID, author, content); err != nil {
		var validationErr *model.ValidationError
		if errors.As(err, &validationErr) {
			h.renderEdit(w, r, viewmodel.MessageEditPage{
				SyncID: syncID, Author: author, Content: content, Error: "Author and content are required.",
			}, http.StatusBadRequest)
			return
		}
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *MessagesHandler) renderEdit(w http.ResponseWriter, r *http.Request, page viewmodel.MessageEditPage, status int) {
	var err error
	if status == http.StatusOK {
		err = h.renderer.RenderPage(w, r, "message_edit", page)
	} else {
		err = h.renderer.RenderWithStatus(w, r, "message_edit", page, status)
	}
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *MessagesHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	pageSize := min(max(getIntParam(r, "limit", 10), 1), 100)
	if !r.URL.Query().Has("limit") && r.URL.Query().Has("pageSize") {
		pageSize = min(max(getIntParam(r, "pageSize", 10), 1), 100)
	}
	offset := max(getIntParam(r, "offset", 0), 0)
	if !r.URL.Query().Has("offset") && r.URL.Query().Has("page") {
		offset = (max(getIntParam(r, "page", 1), 1) - 1) * pageSize
	}

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

	messageItems, err := buildMessageItems(r.Context(), result.Items, h.voteService, user.ID, web.CSRFTokenFromRequest(r), web.LanguageFromRequest(r))
	if err != nil {
		_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
			StatusCode: http.StatusInternalServerError,
			Title:      "Error",
			Message:    "Failed to load message votes",
		}, http.StatusInternalServerError)
		return
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: result.Metadata.CurrentPage,
		TotalPages:  result.Metadata.TotalPages,
		TotalItems:  result.Metadata.TotalItems,
		HasPrevious: result.Metadata.HasPrevious,
		HasNext:     result.Metadata.HasNext,
		PageSize:    result.Metadata.PageSize,
		Language:    web.LanguageFromRequest(r),
	}
	if pagination.HasPrevious {
		pagination.PreviousURL = messageListURL(query, year, pageSize, max(offset-pageSize, 0))
	}
	if pagination.HasNext {
		pagination.NextURL = messageListURL(query, year, pageSize, offset+pageSize)
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
		RefreshURL: messageComponentURL(query, year, pageSize, offset, web.LanguageFromRequest(r), false),
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// Create reads author and content from a form POST and creates a new
// server-originated message, then redirects back to the message list.
func (h *MessagesHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	author := r.FormValue("author")
	content := r.FormValue("content")

	if _, err := h.messageService.CreateServerMessage(r.Context(), author, content); err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Delete soft-deletes a message identified by the {syncId} URL parameter.
func (h *MessagesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	if err := h.messageService.DeleteMessage(r.Context(), syncID); err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Restore un-deletes a soft-deleted message identified by the {syncId} URL
// parameter.
func (h *MessagesHandler) Restore(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	if err := h.messageService.Restore(r.Context(), syncID); err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/messages/trash", http.StatusSeeOther)
}

// Resolve renders the local and server versions of a conflicted message so
// the user can make an informed choice before submitting a resolution.
func (h *MessagesHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")
	message, err := h.messageService.FindBySyncID(r.Context(), syncID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if message.SyncConflict == nil {
		writeError(w, http.StatusConflict, "Message has no sync conflict")
		return
	}

	var server model.SyncMessage
	if err := json.Unmarshal([]byte(*message.SyncConflict), &server); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read the server version")
		return
	}

	if err := h.renderer.RenderPage(w, r, "message_conflict", viewmodel.MessageConflictPage{
		SyncID:        syncID,
		MyAuthor:      message.Author,
		MyContent:     message.Content,
		ServerAuthor:  server.Author,
		ServerContent: server.Content,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// ResolveConflict resolves a sync conflict on the message identified by the
// {syncId} URL parameter using the strategy submitted in the form body
// ("mine" or "server").
func (h *MessagesHandler) ResolveConflict(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	strategy := model.ConflictStrategyFromString(r.FormValue("strategy"))
	if err := h.messageService.ResolveConflict(r.Context(), syncID, strategy); err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func messageListURL(query string, year, limit, offset int) string {
	return messagePageURL("/", query, year, limit, offset)
}

func messagePageURL(path, query string, year, limit, offset int) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if year > 0 {
		values.Set("year", strconv.Itoa(year))
	}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	return path + "?" + values.Encode()
}

func messageComponentURL(query string, year, limit, offset int, language string, trash bool) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if year > 0 {
		values.Set("year", strconv.Itoa(year))
	}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	if language != "" {
		values.Set("lang", language)
	}
	if trash {
		values.Set("trash", "true")
	}
	return "/components/message-list?" + values.Encode()
}
