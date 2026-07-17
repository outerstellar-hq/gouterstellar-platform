package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type preferenceUpdateCall struct {
	userID   uuid.UUID
	language *string
	theme    *string
	layout   *string
}

type stubPreferenceUpdater struct {
	call *preferenceUpdateCall
	err  error
}

func (s *stubPreferenceUpdater) UpdatePreferences(_ context.Context, userID uuid.UUID, language, theme, layout *string) error {
	s.call = &preferenceUpdateCall{userID: userID, language: language, theme: theme, layout: layout}
	return s.err
}

func TestNavigationPreferencesPersistAndReturnToCurrentPage(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	updater := &stubPreferenceUpdater{}
	handler := NewComponentsHandler(nil, nil, nil, nil, nil, updater)
	form := url.Values{
		"pagePath": {"/reports"}, "lang": {"fr"}, "theme": {"nord"}, "layout": {"compact"},
	}
	request := httptest.NewRequest(http.MethodPost, "/components/navigation/preferences", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	request = web.WithUser(request, &model.User{ID: userID, Username: "operator"})
	response := httptest.NewRecorder()

	handler.UpdateNavigationPreferences(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "/reports", response.Header().Get("HX-Redirect"))
	require.NotNil(t, updater.call)
	assert.Equal(t, userID, updater.call.userID)
	assert.Equal(t, "fr", *updater.call.language)
	assert.Equal(t, "nord", *updater.call.theme)
	assert.Equal(t, "compact", *updater.call.layout)
}

func TestNavigationPreferencesRejectInvalidOrAnonymousUpdates(t *testing.T) {
	t.Parallel()

	updater := &stubPreferenceUpdater{}
	handler := NewComponentsHandler(nil, nil, nil, nil, nil, updater)
	anonymous := httptest.NewRecorder()
	handler.UpdateNavigationPreferences(anonymous, httptest.NewRequest(http.MethodPost, "/components/navigation/preferences", nil))
	assert.Equal(t, http.StatusUnauthorized, anonymous.Code)

	form := url.Values{"theme": {"not-a-theme"}}
	request := httptest.NewRequest(http.MethodPost, "/components/navigation/preferences", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = web.WithUser(request, &model.User{ID: uuid.New()})
	invalid := httptest.NewRecorder()
	handler.UpdateNavigationPreferences(invalid, request)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
	assert.Nil(t, updater.call)
}

func TestSafePagePathRejectsExternalRedirects(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/", safePagePath("https://attacker.example"))
	assert.Equal(t, "/", safePagePath("//attacker.example"))
	assert.Equal(t, "/contacts", safePagePath("/contacts"))
}
