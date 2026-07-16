package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	ctx.Routes.Public(http.MethodGet, "/components/footer-status", "Footer status fragment", http.HandlerFunc(h.FooterStatus))
	ctx.Routes.Public(http.MethodGet, "/components/navigation/page", "Theme, language, and layout refresh", http.HandlerFunc(h.NavigationPage))
	ctx.Routes.Public(http.MethodGet, "/components/sidebar/theme-selector", "Theme selector", http.HandlerFunc(h.ThemeSelector))
	ctx.Routes.Public(http.MethodGet, "/components/sidebar/language-selector", "Language selector", http.HandlerFunc(h.LanguageSelector))
	ctx.Routes.Public(http.MethodGet, "/components/sidebar/layout-selector", "Layout selector", http.HandlerFunc(h.LayoutSelector))
	ctx.Routes.Public(http.MethodGet, "/components/messages/{syncId}/vote", "Message vote controls", http.HandlerFunc(h.VoteControls))
	ctx.Routes.Protected(http.MethodPost, "/components/messages/{syncId}/vote", "Vote on message", http.HandlerFunc(h.Vote))
	ctx.Routes.Public(http.MethodGet, "/components/polls/{syncId}", "Poll card", http.HandlerFunc(h.PollCard))
	ctx.Routes.Protected(http.MethodPost, "/components/polls/{syncId}/vote", "Cast poll vote", http.HandlerFunc(h.PollVote))
	ctx.Routes.Protected(http.MethodDelete, "/components/polls/{syncId}/vote", "Remove poll vote", http.HandlerFunc(h.PollRemoveVote))
	ctx.Routes.Protected(http.MethodPost, "/components/polls/{syncId}/remove-vote", "Remove poll vote", http.HandlerFunc(h.PollRemoveVote))
	return nil
}

func (h *ComponentsHandler) FooterStatus(w http.ResponseWriter, r *http.Request) {
	messageCount, err := h.messageService.CountMessages(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load footer status")
		return
	}
	dirtyCount, err := h.messageService.CountDirtyMessages(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load footer status")
		return
	}
	if err := h.renderer.RenderPartial(w, "footer_status", viewmodel.FooterStatus{
		Text: web.TranslateForTemplate(web.LanguageFromRequest(r), "web.footer.status", messageCount, dirtyCount),
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *ComponentsHandler) NavigationPage(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	pagePath := values.Get("pagePath")
	values.Del("pagePath")
	if pagePath == "" || !strings.HasPrefix(pagePath, "/") || strings.HasPrefix(pagePath, "//") || strings.HasPrefix(pagePath, `/\`) {
		pagePath = "/"
	}
	redirectURL := pagePath
	if query := values.Encode(); query != "" {
		redirectURL += "?" + query
	}
	w.Header().Set("HX-Redirect", redirectURL)
	w.WriteHeader(http.StatusOK)
}

func (h *ComponentsHandler) ThemeSelector(w http.ResponseWriter, r *http.Request) {
	h.renderSelector(w, "sidebar_selector", selectorFor(r, "web.sidebar.themes", "web.sidebar.theme.label", "theme", themeOptions))
}

func (h *ComponentsHandler) LanguageSelector(w http.ResponseWriter, r *http.Request) {
	h.renderSelector(w, "sidebar_selector", selectorFor(r, "web.sidebar.language", "web.sidebar.language.label", "lang", languageOptions))
}

func (h *ComponentsHandler) LayoutSelector(w http.ResponseWriter, r *http.Request) {
	h.renderSelector(w, "sidebar_selector", selectorFor(r, "web.sidebar.layout", "web.sidebar.layout.label", "layout", layoutOptions))
}

func (h *ComponentsHandler) renderSelector(w http.ResponseWriter, name string, selector viewmodel.SidebarSelector) {
	if err := h.renderer.RenderPartial(w, name, selector); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

var themeOptions = []viewmodel.SelectorOption{
	{Value: "dark", Label: "Dark"}, {Value: "light", Label: "Light"},
	{Value: "cupcake", Label: "Cupcake"}, {Value: "bumblebee", Label: "Bumblebee"},
	{Value: "emerald", Label: "Emerald"}, {Value: "corporate", Label: "Corporate"},
	{Value: "synthwave", Label: "Synthwave"}, {Value: "retro", Label: "Retro"},
	{Value: "cyberpunk", Label: "Cyberpunk"}, {Value: "valentine", Label: "Valentine"},
	{Value: "halloween", Label: "Halloween"}, {Value: "garden", Label: "Garden"},
	{Value: "forest", Label: "Forest"}, {Value: "aqua", Label: "Aqua"},
	{Value: "lofi", Label: "Lo-Fi"}, {Value: "pastel", Label: "Pastel"},
	{Value: "fantasy", Label: "Fantasy"}, {Value: "wireframe", Label: "Wireframe"},
	{Value: "black", Label: "Black"}, {Value: "luxury", Label: "Luxury"},
	{Value: "dracula", Label: "Dracula"}, {Value: "cmyk", Label: "CMYK"},
	{Value: "autumn", Label: "Autumn"}, {Value: "business", Label: "Business"},
	{Value: "acid", Label: "Acid"}, {Value: "lemonade", Label: "Lemonade"},
	{Value: "night", Label: "Night"}, {Value: "coffee", Label: "Coffee"},
	{Value: "winter", Label: "Winter"}, {Value: "dim", Label: "Dim"},
	{Value: "nord", Label: "Nord"}, {Value: "sunset", Label: "Sunset"},
}

var languageOptions = []viewmodel.SelectorOption{{Value: "en", Label: "English"}, {Value: "fr", Label: "French"}}
var layoutOptions = []viewmodel.SelectorOption{
	{Value: "nice", Label: "Nice"}, {Value: "cozy", Label: "Cozy"}, {Value: "compact", Label: "Compact"},
}

var selectorOptionTranslationKeys = map[string]map[string]string{
	"lang": {
		"en": "web.language.english",
		"fr": "web.language.french",
	},
	"layout": {
		"nice":    "web.layout.nice",
		"cozy":    "web.layout.cozy",
		"compact": "web.layout.compact",
	},
}

func selectorFor(r *http.Request, headingKey, labelKey, name string, options []viewmodel.SelectorOption) viewmodel.SidebarSelector {
	language := web.LanguageFromRequest(r)
	current := r.URL.Query().Get(name)
	user := web.UserFromRequest(r)
	if current == "" && user != nil {
		switch name {
		case "theme":
			if user.Theme != nil {
				current = *user.Theme
			}
		case "lang":
			if user.Language != nil {
				current = *user.Language
			}
		case "layout":
			if user.Layout != nil {
				current = *user.Layout
			}
		}
	}
	if current == "" {
		current = options[0].Value
	}
	selected := localizedSelectedOptions(options, current, name, language)
	pagePath := r.URL.Query().Get("pagePath")
	if pagePath == "" {
		pagePath = "/"
	}
	hidden := url.Values{"pagePath": []string{pagePath}}
	for _, key := range []string{"theme", "lang", "layout"} {
		if key != name {
			if value := r.URL.Query().Get(key); value != "" {
				hidden.Set(key, value)
			}
		}
	}
	return viewmodel.SidebarSelector{
		Heading:    web.TranslateForTemplate(language, headingKey),
		Label:      web.TranslateForTemplate(language, labelKey),
		ApplyLabel: web.TranslateForTemplate(language, "web.common.apply"),
		Name:       name,
		Options:    selected,
		Hidden:     hidden,
	}
}

func selectedOptions(options []viewmodel.SelectorOption, current string) []viewmodel.SelectorOption {
	selected := make([]viewmodel.SelectorOption, len(options))
	copy(selected, options)
	for i := range selected {
		selected[i].Selected = selected[i].Value == current
	}
	return selected
}

func localizedSelectedOptions(options []viewmodel.SelectorOption, current, name, language string) []viewmodel.SelectorOption {
	selected := selectedOptions(options, current)
	for i := range selected {
		if key := selectorOptionTranslationKeys[name][selected[i].Value]; key != "" {
			selected[i].Label = web.TranslateForTemplate(language, key)
		}
	}
	return selected
}

func validOption(options []viewmodel.SelectorOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func (h *ComponentsHandler) MessageList(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
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
		writeError(w, http.StatusInternalServerError, "Failed to load messages")
		return
	}

	messageItems, err := buildMessageItems(r.Context(), result.Items, h.voteService, user.ID, web.CSRFTokenFromRequest(r), web.LanguageFromRequest(r))
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
		Language:    web.LanguageFromRequest(r),
	}
	if pagination.HasPrevious {
		pagination.PreviousURL = messageListURL(query, year, pageSize, max(offset-pageSize, 0))
	}
	if pagination.HasNext {
		pagination.NextURL = messageListURL(query, year, pageSize, offset+pageSize)
	}

	if err := h.renderer.RenderPartial(w, "message_list", viewmodel.MessagesPage{
		Messages:   messageItems,
		Pagination: pagination,
		Query:      query,
		Year:       year,
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
		Language:     web.LanguageFromRequest(r),
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
		Language: web.LanguageFromRequest(r),
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

	if err := h.renderer.RenderPartial(w, "contact_list", viewmodel.ContactsPage{
		Contacts:   contactItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
