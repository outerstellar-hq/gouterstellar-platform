package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

const (
	defaultSearchLimit   = 20
	maxSearchLimit       = 100
	maxSearchSubtitleLen = 120
	searchTypeAll        = "all"
	searchTypeContact    = "contact"
	searchTypeMessage    = "message"
)

type searchProvider struct {
	resultType string
	search     func(context.Context, string, int32) ([]viewmodel.SearchResult, error)
}

type SearchHandler struct {
	messageService *service.MessageService
	contactService *service.ContactService
	renderer       *web.Renderer
}

func NewSearchHandler(
	msgSvc *service.MessageService,
	contactSvc *service.ContactService,
	renderer *web.Renderer,
) *SearchHandler {
	return &SearchHandler{
		messageService: msgSvc,
		contactService: contactSvc,
		renderer:       renderer,
	}
}

// ContributeRoutes registers authenticated HTML and JSON search surfaces.
func (h *SearchHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Protected(http.MethodGet, "/search", "Search", http.HandlerFunc(h.Search))
	ctx.Routes.API(http.MethodGet, "/api/v1/search", "Search messages and contacts", http.HandlerFunc(h.SearchAPI))
	return nil
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	typeFilter := normalizeSearchType(r.URL.Query().Get("type"))
	results, err := h.search(r.Context(), query, typeFilter, searchLimit(r))
	if err != nil {
		h.renderSearchError(w, r)
		return
	}

	page := viewmodel.SearchPage{
		Query:       query,
		Results:     results,
		TypeFilter:  typeFilter,
		TypeFilters: buildSearchTypeFilters(query, typeFilter, web.LanguageFromRequest(r)),
	}
	if err := h.renderer.RenderPage(w, r, "search", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *SearchHandler) SearchAPI(w http.ResponseWriter, r *http.Request) {
	if web.UserFromRequest(r) == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	results, err := h.search(r.Context(), query, normalizeSearchType(r.URL.Query().Get("type")), searchLimit(r))
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query":   query,
		"results": results,
		"total":   len(results),
	})
}

func (h *SearchHandler) search(
	ctx context.Context,
	query string,
	typeFilter string,
	limit int,
) ([]viewmodel.SearchResult, error) {
	if query == "" {
		return []viewmodel.SearchResult{}, nil
	}

	providers := h.providers(typeFilter)
	if len(providers) == 0 {
		return []viewmodel.SearchResult{}, nil
	}

	providerLimit := safeInt32((limit + len(providers) - 1) / len(providers))
	results := make([]viewmodel.SearchResult, 0, limit)
	for _, provider := range providers {
		found, err := provider.search(ctx, query, providerLimit)
		if err != nil {
			return nil, fmt.Errorf("search %s: %w", provider.resultType, err)
		}
		results = append(results, found...)
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (h *SearchHandler) providers(typeFilter string) []searchProvider {
	providers := []searchProvider{
		{resultType: searchTypeMessage, search: h.searchMessages},
		{resultType: searchTypeContact, search: h.searchContacts},
	}
	if typeFilter == searchTypeAll {
		return providers
	}
	for _, provider := range providers {
		if provider.resultType == typeFilter {
			return []searchProvider{provider}
		}
	}
	return nil
}

func (h *SearchHandler) searchMessages(
	ctx context.Context,
	query string,
	limit int32,
) ([]viewmodel.SearchResult, error) {
	page, err := h.messageService.SearchMessages(ctx, query, limit, 0)
	if err != nil {
		return nil, err
	}
	results := make([]viewmodel.SearchResult, len(page.Items))
	for i, message := range page.Items {
		score := 0.8
		if containsFold(message.Content, query) {
			score = 1
		}
		results[i] = viewmodel.SearchResult{
			ID:       message.SyncID,
			Title:    message.Author,
			Subtitle: truncateSearchSubtitle(message.Content),
			URL:      "/?q=" + url.QueryEscape(query),
			Type:     searchTypeMessage,
			Score:    score,
		}
	}
	return results, nil
}

func (h *SearchHandler) searchContacts(
	ctx context.Context,
	query string,
	limit int32,
) ([]viewmodel.SearchResult, error) {
	contacts, _, err := h.contactService.SearchContacts(ctx, query, limit, 0)
	if err != nil {
		return nil, err
	}
	results := make([]viewmodel.SearchResult, len(contacts))
	for i, contact := range contacts {
		score := 0.7
		if containsFold(contact.Name, query) {
			score = 1
		}
		results[i] = viewmodel.SearchResult{
			ID:       contact.SyncID,
			Title:    contact.Name,
			Subtitle: contactSearchSubtitle(contact.Emails, contact.Company, contact.Department),
			URL:      "/contacts/" + url.PathEscape(contact.SyncID),
			Type:     searchTypeContact,
			Score:    score,
		}
	}
	return results, nil
}

func (h *SearchHandler) renderSearchError(w http.ResponseWriter, r *http.Request) {
	_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
		StatusCode: http.StatusInternalServerError,
		Title:      "Error",
		Message:    "Search failed",
	}, http.StatusInternalServerError)
}

func buildSearchTypeFilters(query, active, language string) []viewmodel.SearchTypeFilter {
	escapedQuery := url.QueryEscape(query)
	filters := []struct {
		value string
		label string
	}{
		{value: searchTypeAll, label: web.TranslateForTemplate(language, "web.search.filter.all")},
		{value: searchTypeMessage, label: web.TranslateForTemplate(language, "web.search.filter.message")},
		{value: searchTypeContact, label: web.TranslateForTemplate(language, "web.search.filter.contact")},
	}
	result := make([]viewmodel.SearchTypeFilter, len(filters))
	for i, filter := range filters {
		filterURL := "/search"
		if query != "" {
			filterURL += "?q=" + escapedQuery
			if filter.value != searchTypeAll {
				filterURL += "&type=" + filter.value
			}
		}
		result[i] = viewmodel.SearchTypeFilter{
			Value:  filter.value,
			Label:  filter.label,
			URL:    filterURL,
			Active: active == filter.value,
		}
	}
	return result
}

func searchLimit(r *http.Request) int {
	limit := getIntParam(r, "limit", defaultSearchLimit)
	if limit < 1 {
		return 1
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

func normalizeSearchType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return searchTypeAll
	}
	return value
}

func contactSearchSubtitle(emails []string, company, department string) string {
	if len(emails) > 0 {
		return emails[0]
	}
	if company != "" {
		return company
	}
	return department
}

func truncateSearchSubtitle(value string) string {
	runes := []rune(value)
	if len(runes) <= maxSearchSubtitleLen {
		return value
	}
	return string(runes[:maxSearchSubtitleLen]) + "…"
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}
