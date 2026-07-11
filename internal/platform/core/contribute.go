package core

import (
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

// Contribute registers every core route through the platform's route registry.
// It maps each Bundle field to a route group (Public/Protected/API/Admin) so
// the platform's wire layer can apply the correct middleware and the route
// registry can validate ownership.
func (e *Extension) Contribute(ctx *extplatform.ContributionContext) error {
	b := e.bundle

	// --- Public UI (no auth required) ---
	ctx.Routes.Public(http.MethodGet, "/auth", "Login page", http.HandlerFunc(b.AuthShowLogin))
	ctx.Routes.Public(http.MethodPost, "/auth/login", "Handle login", http.HandlerFunc(b.AuthHandleLogin))
	ctx.Routes.Public(http.MethodPost, "/auth/register", "Handle registration", http.HandlerFunc(b.AuthHandleRegister))
	ctx.Routes.Public(http.MethodPost, "/auth/logout", "Handle logout", http.HandlerFunc(b.AuthHandleLogout))
	ctx.Routes.Public(http.MethodGet, "/auth/change-password", "Change password page", http.HandlerFunc(b.AuthShowChangePwd))
	ctx.Routes.Public(http.MethodPost, "/auth/change-password", "Handle password change", http.HandlerFunc(b.AuthHandleChangePwd))
	ctx.Routes.Public(http.MethodGet, "/auth/reset", "Reset password page", http.HandlerFunc(b.AuthShowReset))
	ctx.Routes.Public(http.MethodPost, "/auth/reset", "Handle password reset", http.HandlerFunc(b.AuthHandleReset))
	ctx.Routes.Public(http.MethodGet, "/auth/oauth/{provider}", "OAuth redirect", http.HandlerFunc(b.OAuthRedirect))
	ctx.Routes.Public(http.MethodGet, "/auth/oauth/{provider}/callback", "OAuth callback", http.HandlerFunc(b.OAuthCallback))
	ctx.Routes.Public(http.MethodPost, "/auth/oauth/{provider}/callback", "OAuth callback POST", http.HandlerFunc(b.OAuthCallbackPost))

	// --- Health / metrics / static assets ---
	// /health is public and unauthenticated so orchestrators can probe it.
	ctx.Routes.Public(http.MethodGet, "/health", "Health check", http.HandlerFunc(b.Health))
	ctx.Routes.API(http.MethodGet, "/metrics", "Prometheus metrics", b.Metrics)
	ctx.Routes.Assets("/static/*", b.Static)

	// --- Protected UI (auth required) ---
	ctx.Routes.Protected(http.MethodGet, "/", "Home dashboard", http.HandlerFunc(b.HomeShow))
	ctx.Routes.Protected(http.MethodGet, "/contacts", "Contacts list", http.HandlerFunc(b.ContactsList))
	ctx.Routes.Protected(http.MethodGet, "/contacts/{syncId}", "Contact detail", http.HandlerFunc(b.ContactsDetail))
	ctx.Routes.Protected(http.MethodPost, "/contacts/create", "Create contact", http.HandlerFunc(b.ContactsCreate))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/update", "Update contact", http.HandlerFunc(b.ContactsUpdate))
	ctx.Routes.Protected(http.MethodPost, "/contacts/{syncId}/delete", "Delete contact", http.HandlerFunc(b.ContactsDelete))
	ctx.Routes.Protected(http.MethodGet, "/search", "Search", http.HandlerFunc(b.SearchSearch))
	ctx.Routes.Protected(http.MethodGet, "/settings", "Settings page", http.HandlerFunc(b.SettingsShow))
	ctx.Routes.Protected(http.MethodPost, "/settings/profile", "Update profile", http.HandlerFunc(b.SettingsProfile))
	ctx.Routes.Protected(http.MethodPost, "/settings/password", "Change password", http.HandlerFunc(b.SettingsPassword))
	ctx.Routes.Protected(http.MethodPost, "/settings/preferences", "Update preferences", http.HandlerFunc(b.SettingsPreferences))
	ctx.Routes.Protected(http.MethodPost, "/settings/api-keys", "Create API key", http.HandlerFunc(b.SettingsCreateAPIKey))
	ctx.Routes.Protected(http.MethodPost, "/settings/api-keys/{id}/delete", "Delete API key", http.HandlerFunc(b.SettingsDeleteAPIKey))
	ctx.Routes.Protected(http.MethodPost, "/settings/notifications", "Update notification prefs", http.HandlerFunc(b.SettingsNotifPrefs))
	ctx.Routes.Protected(http.MethodGet, "/notifications", "Notifications list", http.HandlerFunc(b.NotifsList))
	ctx.Routes.Protected(http.MethodPost, "/notifications/{id}/read", "Mark notification read", http.HandlerFunc(b.NotifsMarkRead))
	ctx.Routes.Protected(http.MethodPost, "/notifications/read-all", "Mark all read", http.HandlerFunc(b.NotifsMarkAllRead))
	ctx.Routes.Protected(http.MethodPost, "/notifications/{id}/delete", "Delete notification", http.HandlerFunc(b.NotifsDelete))
	ctx.Routes.Protected(http.MethodGet, "/components/message-list", "Message list partial", http.HandlerFunc(b.ComponentsMsgList))
	ctx.Routes.Protected(http.MethodGet, "/components/contact-list", "Contact list partial", http.HandlerFunc(b.ComponentsContactList))
	ctx.Routes.Protected(http.MethodGet, "/ws/sync", "WebSocket sync", http.HandlerFunc(b.SyncWebSocket))

	// --- API (bearer auth applied by builder) ---
	ctx.Routes.API(http.MethodGet, "/api/v1/sync", "Pull message changes", http.HandlerFunc(b.SyncPullMessages))
	ctx.Routes.API(http.MethodPost, "/api/v1/sync", "Push message changes", http.HandlerFunc(b.SyncPushMessages))
	ctx.Routes.API(http.MethodGet, "/api/v1/sync/contacts", "Pull contact changes", http.HandlerFunc(b.SyncPullContacts))
	ctx.Routes.API(http.MethodPost, "/api/v1/sync/contacts", "Push contact changes", http.HandlerFunc(b.SyncPushContacts))

	ctx.Routes.API(http.MethodPost, "/api/v1/auth/login", "API login", http.HandlerFunc(b.AuthAPILogin))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/token", "Issue token", http.HandlerFunc(b.AuthAPIToken))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/register", "API register", http.HandlerFunc(b.AuthAPIRegister))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/change-password", "API change password", http.HandlerFunc(b.AuthAPIChangePwd))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/reset-password", "Request password reset", http.HandlerFunc(b.AuthAPIResetReq))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/confirm-reset", "Confirm password reset", http.HandlerFunc(b.AuthAPIConfirmReset))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/logout", "API logout", http.HandlerFunc(b.AuthAPILogout))
	ctx.Routes.API(http.MethodGet, "/api/v1/auth/profile", "Get profile", http.HandlerFunc(b.AuthAPIGetProfile))
	ctx.Routes.API(http.MethodPut, "/api/v1/auth/profile", "Update profile", http.HandlerFunc(b.AuthAPIUpdateProfile))
	ctx.Routes.API(http.MethodPut, "/api/v1/auth/notification-preferences", "Update notif prefs", http.HandlerFunc(b.AuthAPINotifPrefs))
	ctx.Routes.API(http.MethodDelete, "/api/v1/auth/account", "Delete account", http.HandlerFunc(b.AuthAPIDeleteAccount))
	ctx.Routes.API(http.MethodPost, "/api/v1/auth/api-keys", "Create API key", http.HandlerFunc(b.AuthAPICreateAPIKey))
	ctx.Routes.API(http.MethodGet, "/api/v1/auth/api-keys", "List API keys", http.HandlerFunc(b.AuthAPIListAPIKeys))
	ctx.Routes.API(http.MethodDelete, "/api/v1/auth/api-keys/{id}", "Delete API key", http.HandlerFunc(b.AuthAPIDeleteAPIKey))

	ctx.Routes.API(http.MethodGet, "/api/v1/users", "List users", http.HandlerFunc(b.UserAPIListUsers))
	ctx.Routes.API(http.MethodGet, "/api/v1/users/count", "Count users", http.HandlerFunc(b.UserAPICountUsers))
	ctx.Routes.API(http.MethodPut, "/api/v1/users/{id}/enabled", "Set user enabled", http.HandlerFunc(b.UserAPISetEnabled))
	ctx.Routes.API(http.MethodPut, "/api/v1/users/{id}/role", "Set user role", http.HandlerFunc(b.UserAPISetRole))
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/users/export", "Export users CSV", http.HandlerFunc(b.UserAPIExportUsers))
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/audit/export", "Export audit CSV", http.HandlerFunc(b.UserAPIExportAudit))

	ctx.Routes.API(http.MethodGet, "/api/v1/notifications", "List notifications", http.HandlerFunc(b.NotifAPIList))
	ctx.Routes.API(http.MethodGet, "/api/v1/notifications/unread-count", "Unread count", http.HandlerFunc(b.NotifAPIUnreadCount))
	ctx.Routes.API(http.MethodPut, "/api/v1/notifications/{id}/read", "Mark read", http.HandlerFunc(b.NotifAPIMarkRead))
	ctx.Routes.API(http.MethodPut, "/api/v1/notifications/read-all", "Mark all read", http.HandlerFunc(b.NotifAPIMarkAllRead))
	ctx.Routes.API(http.MethodDelete, "/api/v1/notifications/{id}", "Delete notification", http.HandlerFunc(b.NotifAPIDelete))

	ctx.Routes.API(http.MethodPost, "/api/v1/devices/register", "Register device", http.HandlerFunc(b.DeviceAPIRegister))
	ctx.Routes.API(http.MethodDelete, "/api/v1/devices/{id}", "Unregister device", http.HandlerFunc(b.DeviceAPIUnregister))

	// --- Admin ---
	ctx.Routes.Admin(http.MethodGet, "/admin/users", "User management", http.HandlerFunc(b.AdminListUsers))
	ctx.Routes.Admin(http.MethodPost, "/admin/users/{id}/enabled", "Set user enabled", http.HandlerFunc(b.AdminSetEnabled))
	ctx.Routes.Admin(http.MethodPost, "/admin/users/{id}/role", "Set user role", http.HandlerFunc(b.AdminSetRole))
	ctx.Routes.Admin(http.MethodGet, "/admin/users/export", "Export users", http.HandlerFunc(b.AdminExportUsers))
	ctx.Routes.Admin(http.MethodGet, "/admin/audit", "Audit log", http.HandlerFunc(b.AdminShowAudit))
	ctx.Routes.Admin(http.MethodGet, "/admin/audit/export", "Export audit", http.HandlerFunc(b.AdminExportAudit))

	if b.DevDashboardEnabled {
		ctx.Routes.Admin(http.MethodGet, "/dev/dashboard", "Dev dashboard", http.HandlerFunc(b.DevDashboard))
		ctx.Routes.Admin(http.MethodPost, "/dev/outbox/process", "Process outbox", http.HandlerFunc(b.DevProcessOutbox))
		ctx.Routes.Admin(http.MethodPost, "/dev/sessions/cleanup", "Cleanup sessions", http.HandlerFunc(b.DevCleanupSessions))
		ctx.Routes.Admin(http.MethodPost, "/dev/cache/invalidate", "Invalidate cache", http.HandlerFunc(b.DevInvalidateCache))
	}

	// --- Navigation ---
	ctx.Navigation.Add("Home", "/", "house")
	ctx.Navigation.Add("Contacts", "/contacts", "users")
	ctx.Navigation.Add("Search", "/search", "search")
	ctx.Navigation.Add("Settings", "/settings", "gear")
	ctx.Navigation.Add("Notifications", "/notifications", "bell")

	return nil
}
