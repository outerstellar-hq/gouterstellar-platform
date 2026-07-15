package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
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
	ctx.Routes.Protected(http.MethodGet, "/messages", "Messages", http.HandlerFunc(h.Show))
	ctx.Routes.Protected(http.MethodPost, "/messages/create", "Create message", http.HandlerFunc(h.Create))
	ctx.Routes.Protected(http.MethodPost, "/messages/{syncId}/delete", "Delete message", http.HandlerFunc(h.Delete))
	ctx.Routes.Protected(http.MethodPost, "/messages/{syncId}/restore", "Restore message", http.HandlerFunc(h.Restore))
	ctx.Routes.Protected(http.MethodPost, "/messages/{syncId}/resolve", "Resolve conflict", http.HandlerFunc(h.ResolveConflict))
	return nil
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

	messageItems, err := buildMessageItems(r.Context(), result.Items, h.voteService, user.ID, web.CSRFTokenFromRequest(r))
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

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

// Delete soft-deletes a message identified by the {syncId} URL parameter.
func (h *MessagesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	syncID := chi.URLParam(r, "syncId")

	if err := h.messageService.DeleteMessage(r.Context(), syncID); err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
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

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}
