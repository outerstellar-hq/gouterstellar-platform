package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"template/base.html": &fstest.MapFile{
			Data: []byte(`{{ define "base" }}<html><head><title>{{ .Title }}</title></head><body><nav>NAV</nav><main>{{ template "content" . }}</main></body></html>{{ end }}`),
		},
		"template/partials/pagination.html": &fstest.MapFile{
			Data: []byte(`{{ define "pagination" }}<div class="pagination">page {{ .CurrentPage }}</div>{{ end }}`),
		},
		"template/pages/home.html": &fstest.MapFile{
			Data: []byte(`{{ define "content" }}<h1>{{ .BodyData.Title }}</h1>{{ end }}`),
		},
	}
}

func TestNewRendererParsesAllPages(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	assert.Contains(t, r.pages, "home", "home page should be parsed")
}

func TestRenderPageProducesShellAndContent(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err = r.RenderPage(rec, req, "home", map[string]string{"Title": "Welcome"})
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "<nav>NAV</nav>", "shell nav should be present")
	assert.Contains(t, body, "<h1>Welcome</h1>", "page content should be present")
}

func TestRenderPartialProducesFragment(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	err = r.RenderPartial(rec, "pagination", map[string]int{"CurrentPage": 3})
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "page 3")
	assert.NotContains(t, body, "<nav>", "partial should not contain shell chrome")
}

func TestRenderPageSetsContentType(t *testing.T) {
	r, err := NewRenderer(testFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	_ = r.RenderPage(rec, req, "home", map[string]string{"Title": "X"})

	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestAdminUsersRendersLockedAccountControl(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)

	err = r.RenderPage(rec, req, "admin_users", viewmodel.AdminUsersPage{
		Users: []viewmodel.UserItem{{
			ID:                  "user-id",
			Username:            "alice",
			Role:                "USER",
			Enabled:             true,
			FailedLoginAttempts: 10,
			IsLocked:            true,
		}},
	})

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "Locked")
	assert.Contains(t, rec.Body.String(), "/admin/users/user-id/unlock")
	assert.Contains(t, rec.Body.String(), "Unlock (10)")
}

func TestLoginRendersTOTPChallenge(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)

	err = r.RenderPage(rec, req, "auth_login", viewmodel.AuthPage{
		TOTPRequired: true,
		PartialToken: "pt_example",
	})

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "/auth/totp/verify")
	assert.Contains(t, rec.Body.String(), "pt_example")
	assert.Contains(t, rec.Body.String(), "autocomplete=\"one-time-code\"")
}

func TestSettingsRendersTOTPEnrollmentAndBackupCodes(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/settings?tab=security", nil)

	setupRec := httptest.NewRecorder()
	err = r.RenderPage(setupRec, req, "settings", viewmodel.SettingsPage{
		ActiveTab: "security",
		TOTPSetup: &viewmodel.TOTPSetupData{Secret: "SECRET", QRDataURI: "data:image/png;base64,AAAA"},
	})
	require.NoError(t, err)
	assert.Contains(t, setupRec.Body.String(), "QR code for setting up two-factor authentication")
	assert.Contains(t, setupRec.Body.String(), "SECRET")

	backupRec := httptest.NewRecorder()
	err = r.RenderPage(backupRec, req, "settings", viewmodel.SettingsPage{
		ActiveTab:       "security",
		TOTPEnabled:     true,
		TOTPBackupCodes: []string{"BACKUP-CODE"},
	})
	require.NoError(t, err)
	assert.Contains(t, backupRec.Body.String(), "BACKUP-CODE")
	assert.Contains(t, backupRec.Body.String(), "will not be shown again")
}
