package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/viewmodel"
)

var templateTranslationKeyPattern = regexp.MustCompile(`translate\s+[^\s}]+\s+"([^"]+)"`)

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

func TestRegisterTemplatesRejectsPageCollisionWithBothOwners(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "test")
	require.NoError(t, err)

	first := fstest.MapFS{
		"pages/extension.html": &fstest.MapFile{Data: []byte(`{{ define "content" }}first{{ end }}`)},
	}
	require.NoError(t, renderer.RegisterTemplates("first-extension", first, "pages", ""))

	second := fstest.MapFS{
		"pages/extension.html": &fstest.MapFile{Data: []byte(`{{ define "content" }}second{{ end }}`)},
	}
	err = renderer.RegisterTemplates("second-extension", second, "pages", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "second-extension")
	assert.Contains(t, err.Error(), "first-extension")
}

func TestRegisterTemplatesRejectsCorePartialCollision(t *testing.T) {
	renderer, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "test")
	require.NoError(t, err)

	source := fstest.MapFS{
		"pages/extension.html":    &fstest.MapFile{Data: []byte(`{{ define "content" }}extension{{ end }}`)},
		"partials/collision.html": &fstest.MapFile{Data: []byte(`{{ define "message_list" }}collision{{ end }}`)},
	}
	err = renderer.RegisterTemplates("colliding-extension", source, "pages", "partials")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "colliding-extension")
	assert.Contains(t, err.Error(), "owner platform")
}

func TestSidebarSelectorOnlyListensToItsOwnChanges(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)

	var output strings.Builder
	err = r.partials.ExecuteTemplate(&output, "sidebar_selector", viewmodel.SidebarSelector{
		Heading:   "Language",
		Label:     "Choose your language",
		Name:      "lang",
		CSRFToken: "selector-csrf",
	})
	require.NoError(t, err)
	assert.Contains(t, output.String(), `hx-trigger="change"`)
	assert.Contains(t, output.String(), `hx-post="/components/navigation/preferences"`)
	assert.Contains(t, output.String(), `name="csrf_token" value="selector-csrf"`)
	assert.NotContains(t, output.String(), `from:select`)
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

func TestRendererUsesJavaAppearanceDefaults(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := WithUser(httptest.NewRequest(http.MethodGet, "/", nil), &model.User{
		ID: uuid.New(), Username: "alice", Role: model.RoleUser,
	})

	err = r.RenderPage(rec, req, "messages", viewmodel.MessagesPage{})

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), `<html lang="en" data-theme="dark">`)
	assert.Contains(t, rec.Body.String(), `class="dark layout-sidebar density-nice"`)
}

func TestRendererUsesJavaFrenchCatalogForShellAndPage(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	language := "fr"
	req := WithUser(httptest.NewRequest(http.MethodGet, "/search", nil), &model.User{
		ID: uuid.New(), Username: "alice", Role: model.RoleUser, Language: &language,
	})
	req = WithNavItems(req, []viewmodel.NavItem{{Label: "Home", URL: "/"}, {Label: "Settings", URL: "/settings"}})
	rec := httptest.NewRecorder()

	err = r.RenderPage(rec, req, "search", viewmodel.SearchPage{})

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, `<title>Recherche - Outerstellar Platform</title>`)
	assert.Contains(t, body, `>Accueil</a>`)
	assert.Contains(t, body, `>Paramètres</a>`)
	assert.Contains(t, body, `>Se déconnecter</button>`)
	assert.Contains(t, body, `Aller au contenu`)
	assert.Contains(t, body, `theme=dark&lang=fr&layout=nice`)
	assert.Contains(t, body, `/components/notification-bell?lang=fr`)
	assert.NotContains(t, body, `\u00`)
}

func TestJavaCatalogUnicodeAndPrintfParameters(t *testing.T) {
	assert.Equal(t, "L’authentification à deux facteurs a été activée", TranslateForTemplate("fr", "web.totp.setupSuccess"))
	assert.Equal(t, "3 codes de secours restants", TranslateForTemplate("fr", "web.totp.backupCodes.remaining", "3"))
}

func TestLanguageFromRequestUsesOnlySupportedOverrides(t *testing.T) {
	french := "fr"
	req := WithUser(httptest.NewRequest(http.MethodGet, "/?lang=xx", nil), &model.User{Language: &french})
	assert.Equal(t, "fr", LanguageFromRequest(req))

	req = httptest.NewRequest(http.MethodGet, "/?lang=fr", nil)
	assert.Equal(t, "fr", LanguageFromRequest(req))
}

func TestTemplateTranslationKeysExistInEnglishAndFrench(t *testing.T) {
	english := localeKeys(t, "locales/en.properties")
	french := localeKeys(t, "locales/fr.properties")
	for key := range english {
		assert.Contains(t, french, key, "French catalog is missing %s", key)
	}
	for key := range french {
		assert.Contains(t, english, key, "English catalog is missing %s", key)
	}

	err := fs.WalkDir(TemplateFS(), "template", func(path string, entry fs.DirEntry, err error) error {
		require.NoError(t, err)
		if entry.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, readErr := fs.ReadFile(TemplateFS(), path)
		require.NoError(t, readErr)
		for _, match := range templateTranslationKeyPattern.FindAllStringSubmatch(string(data), -1) {
			assert.Contains(t, english, match[1], "%s uses missing English key %s", path, match[1])
			assert.Contains(t, french, match[1], "%s uses missing French key %s", path, match[1])
		}
		return nil
	})
	require.NoError(t, err)
}

func localeKeys(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	data, err := fs.ReadFile(LocaleFS, path)
	require.NoError(t, err)
	keys := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if separator := strings.IndexAny(line, "=:"); separator >= 0 {
			keys[strings.TrimSpace(line[:separator])] = struct{}{}
		}
	}
	return keys
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

func TestAdminUsersHidesMutationsForCurrentUser(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)

	err = r.RenderPage(rec, req, "admin_users", viewmodel.AdminUsersPage{
		Users: []viewmodel.UserItem{{ID: "self", Username: "admin", Role: "ADMIN", Enabled: true, IsSelf: true}},
	})

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "(you)")
	assert.NotContains(t, rec.Body.String(), "/admin/users/self/toggle-role")
	assert.NotContains(t, rec.Body.String(), "/admin/users/self/toggle-enabled")
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

func TestLoginRendersRegistrationOnlyWhenEnabled(t *testing.T) {
	r, err := NewRenderer(TemplateFS(), TemplateFuncMap(), "1.0.0")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/auth?mode=register", nil)

	registerRec := httptest.NewRecorder()
	err = r.RenderPage(registerRec, req, "auth_login", viewmodel.AuthPage{
		RegistrationEnabled: true,
		RegisterMode:        true,
		Username:            "alice",
	})
	require.NoError(t, err)
	assert.Contains(t, registerRec.Body.String(), "action=\"/auth/register\"")
	assert.Contains(t, registerRec.Body.String(), "value=\"alice\"")
	assert.Contains(t, registerRec.Body.String(), "Passwords are limited to 72 UTF-8 bytes.")
	assert.Contains(t, registerRec.Body.String(), "autocomplete=\"new-password\"")

	disabledRec := httptest.NewRecorder()
	err = r.RenderPage(disabledRec, req, "auth_login", viewmodel.AuthPage{})
	require.NoError(t, err)
	assert.NotContains(t, disabledRec.Body.String(), "/auth?mode=register")
	assert.NotContains(t, disabledRec.Body.String(), "action=\"/auth/register\"")
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
