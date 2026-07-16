package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type ComponentsHandler struct {
	messageService *service.MessageService
	contactService *service.ContactService
	voteService    messageVoteService
	pollService    pollService
	renderer       *web.Renderer
}

func NewComponentsHandler(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	voteSvc messageVoteService,
	pollSvc pollService,
	renderer *web.Renderer,
) *ComponentsHandler {
	return &ComponentsHandler{
		messageService: msgSvc,
		contactService: contactSvc,
		voteService:    voteSvc,
		pollService:    pollSvc,
		renderer:       renderer,
	}
}

// ContributeRoutes registers the component partial routes (protected).
func (h *ComponentsHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/components/message-list", "Message list partial", http.HandlerFunc(h.MessageList))
	ctx.Routes.Protected(http.MethodGet, "/components/contact-list", "Contact list partial", http.HandlerFunc(h.ContactList))
	ctx.Routes.Public(http.MethodGet, "/components/messages/{syncId}/vote", "Message vote controls", http.HandlerFunc(h.VoteControls))
	ctx.Routes.Protected(http.MethodPost, "/components/messages/{syncId}/vote", "Vote on message", http.HandlerFunc(h.Vote))
	ctx.Routes.Public(http.MethodGet, "/components/polls/{syncId}", "Poll card", http.HandlerFunc(h.PollCard))
	ctx.Routes.Protected(http.MethodPost, "/components/polls/{syncId}/vote", "Cast poll vote", http.HandlerFunc(h.PollVote))
	ctx.Routes.Protected(http.MethodDelete, "/components/polls/{syncId}/vote", "Remove poll vote", http.HandlerFunc(h.PollRemoveVote))
	ctx.Routes.Protected(http.MethodPost, "/components/polls/{syncId}/remove-vote", "Remove poll vote", http.HandlerFunc(h.PollRemoveVote))
	return nil
}

func (h *ComponentsHandler) MessageList(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	result, err := h.messageService.ListMessages(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load messages")
		return
	}

	messageItems, err := buildMessageItems(r.Context(), result.Items, h.voteService, user.ID, web.CSRFTokenFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load message votes")
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

	if err := h.renderer.RenderPartial(w, "message_list", viewmodel.MessagesPage{
		Messages:   messageItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ComponentsHandler) VoteControls(w http.ResponseWriter, r *http.Request) {
	var userID *uuid.UUID
	if user := web.UserFromRequest(r); user != nil {
		userID = &user.ID
	}
	score, err := h.voteService.GetScore(r.Context(), chi.URLParam(r, "syncId"), userID)
	if err != nil {
		handleVoteComponentError(w, err)
		return
	}
	h.renderVoteControls(w, r, score)
}

func (h *ComponentsHandler) Vote(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxVoteRequestBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid vote", http.StatusBadRequest)
		return
	}
	direction, err := strconv.ParseInt(r.FormValue("direction"), 10, 16)
	if err != nil || !model.VoteDirection(direction).Valid() {
		http.Error(w, "direction must be 1 or -1", http.StatusBadRequest)
		return
	}
	score, err := h.voteService.Vote(r.Context(), user.ID, chi.URLParam(r, "syncId"), model.VoteDirection(direction))
	if err != nil {
		handleVoteComponentError(w, err)
		return
	}
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/messages", http.StatusSeeOther)
		return
	}
	h.renderVoteControls(w, r, score)
}

func (h *ComponentsHandler) renderVoteControls(w http.ResponseWriter, r *http.Request, score *model.VoteScore) {
	controls := viewmodel.VoteControls{
		SyncID: score.MessageSyncID, Upvotes: score.Upvotes, Downvotes: score.Downvotes,
		NetScore: score.NetScore, CSRFToken: web.CSRFTokenFromRequest(r),
		HasUpvoted:   score.UserVote != nil && *score.UserVote == model.VoteUp,
		HasDownvoted: score.UserVote != nil && *score.UserVote == model.VoteDown,
	}
	if err := h.renderer.RenderPartial(w, "vote_controls", controls); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func handleVoteComponentError(w http.ResponseWriter, err error) {
	var notFound *model.MessageNotFoundError
	var validation *model.ValidationError
	switch {
	case errors.As(err, &notFound):
		http.Error(w, "Message not found", http.StatusNotFound)
	case errors.As(err, &validation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		slog.Error("Vote component failed", "error", err)
		http.Error(w, "Could not update vote", http.StatusInternalServerError)
	}
}

func (h *ComponentsHandler) PollCard(w http.ResponseWriter, r *http.Request) {
	var userID *uuid.UUID
	if user := web.UserFromRequest(r); user != nil {
		userID = &user.ID
	}
	results, err := h.pollService.Get(r.Context(), chi.URLParam(r, "syncId"), userID)
	if err != nil {
		handlePollComponentError(w, err)
		return
	}
	h.renderPollCard(w, r, results)
}

func (h *ComponentsHandler) PollVote(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPollRequestBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid poll vote", http.StatusBadRequest)
		return
	}
	optionID, err := strconv.ParseInt(r.FormValue("optionId"), 10, 64)
	if err != nil || optionID <= 0 {
		http.Error(w, "optionId must be a positive integer", http.StatusBadRequest)
		return
	}
	results, err := h.pollService.CastVote(r.Context(), chi.URLParam(r, "syncId"), optionID, user.ID)
	if err != nil {
		handlePollComponentError(w, err)
		return
	}
	h.renderPollCard(w, r, results)
}

func (h *ComponentsHandler) PollRemoveVote(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	optionValue := r.URL.Query().Get("optionId")
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, maxPollRequestBytes)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid poll vote", http.StatusBadRequest)
			return
		}
		optionValue = r.FormValue("optionId")
	}
	optionID, err := strconv.ParseInt(optionValue, 10, 64)
	if err != nil || optionID <= 0 {
		http.Error(w, "optionId must be a positive integer", http.StatusBadRequest)
		return
	}
	syncID := chi.URLParam(r, "syncId")
	if err := h.pollService.RemoveVote(r.Context(), syncID, optionID, user.ID); err != nil {
		handlePollComponentError(w, err)
		return
	}
	results, err := h.pollService.Get(r.Context(), syncID, &user.ID)
	if err != nil {
		handlePollComponentError(w, err)
		return
	}
	h.renderPollCard(w, r, results)
}

func (h *ComponentsHandler) renderPollCard(w http.ResponseWriter, r *http.Request, results *model.PollWithResults) {
	closed := results.Poll.IsClosed(time.Now())
	selected := make(map[int64]struct{}, len(results.UserVotedOptionIDs))
	for _, optionID := range results.UserVotedOptionIDs {
		selected[optionID] = struct{}{}
	}
	options := make([]viewmodel.PollOption, len(results.Options))
	for i, option := range results.Options {
		_, isSelected := selected[option.ID]
		percentage := int32(0)
		if results.TotalVotes > 0 {
			percentage = results.VoteCounts[option.ID] * 100 / results.TotalVotes
		}
		options[i] = viewmodel.PollOption{
			ID: option.ID, Text: option.OptionText, VoteCount: results.VoteCounts[option.ID],
			Percent: percentage, Selected: isSelected,
			CanVote: !closed && (results.Poll.MultiChoice || len(selected) == 0),
		}
	}
	deadlineLabel := ""
	if results.Poll.Deadline != nil {
		deadlineLabel = results.Poll.Deadline.UTC().Format("2 Jan 2006, 15:04 UTC")
	}
	card := viewmodel.PollCard{
		SyncID: results.Poll.SyncID, Question: results.Poll.Question, MultiChoice: results.Poll.MultiChoice,
		Closed: closed, DeadlineLabel: deadlineLabel, TotalVotes: results.TotalVotes,
		Options: options, CSRFToken: web.CSRFTokenFromRequest(r),
	}
	if err := h.renderer.RenderPartial(w, "poll_card", card); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func handlePollComponentError(w http.ResponseWriter, err error) {
	var notFound *model.PollNotFoundError
	var validation *model.ValidationError
	var conflict *model.PollConflictError
	switch {
	case errors.As(err, &notFound):
		http.Error(w, "Poll not found", http.StatusNotFound)
	case errors.As(err, &validation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.As(err, &conflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		slog.Error("Poll component failed", "error", err)
		http.Error(w, "Could not update poll", http.StatusInternalServerError)
	}
}

func (h *ComponentsHandler) ContactList(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	contacts, err := h.contactService.ListContacts(r.Context(), safeInt32(pageSize), safeInt32(offset))
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

	if err := h.renderer.RenderPartial(w, "contact_list", viewmodel.ContactsPage{
		Contacts:   contactItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
