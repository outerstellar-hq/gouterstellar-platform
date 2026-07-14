package core

import (
	"embed"
	"net/http"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

// Migrations embeds the SQL migration files shipped by the core extension.
// Files live under internal/platform/core/migrations/*.sql.
//
//go:embed migrations/*.sql
var Migrations embed.FS

// Bundle holds all the platform's internal HTTP handlers as function values.
// It is the bridge between existing handler structs and the core extension.
// Each field corresponds to a method on one of the existing handler structs;
// the handler structs themselves are unchanged.
type Bundle struct {
	// PublicUI handlers
	AuthShowLogin          http.HandlerFunc
	AuthHandleLogin        http.HandlerFunc
	AuthHandleRegister     http.HandlerFunc
	AuthHandleLogout       http.HandlerFunc
	AuthShowChangePwd      http.HandlerFunc
	AuthHandleChangePwd    http.HandlerFunc
	AuthShowReset          http.HandlerFunc
	AuthHandleReset        http.HandlerFunc
	AuthShowConfirmReset   http.HandlerFunc
	AuthHandleConfirmReset http.HandlerFunc
	OAuthRedirect          http.HandlerFunc
	OAuthCallback          http.HandlerFunc
	OAuthCallbackPost      http.HandlerFunc

	// ProtectedUI handlers
	HomeShow              http.HandlerFunc
	MessagesShow          http.HandlerFunc
	ContactsList          http.HandlerFunc
	ContactsDetail        http.HandlerFunc
	ContactsCreate        http.HandlerFunc
	ContactsUpdate        http.HandlerFunc
	ContactsDelete        http.HandlerFunc
	SearchSearch          http.HandlerFunc
	SettingsShow          http.HandlerFunc
	SettingsProfile       http.HandlerFunc
	SettingsPassword      http.HandlerFunc
	SettingsPreferences   http.HandlerFunc
	SettingsCreateAPIKey  http.HandlerFunc
	SettingsDeleteAPIKey  http.HandlerFunc
	SettingsNotifPrefs    http.HandlerFunc
	NotifsList            http.HandlerFunc
	NotifsMarkRead        http.HandlerFunc
	NotifsMarkAllRead     http.HandlerFunc
	NotifsDelete          http.HandlerFunc
	ComponentsMsgList     http.HandlerFunc
	ComponentsContactList http.HandlerFunc
	SyncWebSocket         http.HandlerFunc

	// API handlers
	SyncPullMessages     http.HandlerFunc
	SyncPushMessages     http.HandlerFunc
	SyncPullContacts     http.HandlerFunc
	SyncPushContacts     http.HandlerFunc
	AuthAPILogin         http.HandlerFunc
	AuthAPIToken         http.HandlerFunc
	AuthAPIRegister      http.HandlerFunc
	AuthAPIChangePwd     http.HandlerFunc
	AuthAPIResetReq      http.HandlerFunc
	AuthAPIConfirmReset  http.HandlerFunc
	AuthAPILogout        http.HandlerFunc
	AuthAPIGetProfile    http.HandlerFunc
	AuthAPIUpdateProfile http.HandlerFunc
	AuthAPINotifPrefs    http.HandlerFunc
	AuthAPIDeleteAccount http.HandlerFunc
	AuthAPICreateAPIKey  http.HandlerFunc
	AuthAPIListAPIKeys   http.HandlerFunc
	AuthAPIDeleteAPIKey  http.HandlerFunc
	UserAPIListUsers     http.HandlerFunc
	UserAPICountUsers    http.HandlerFunc
	UserAPISetEnabled    http.HandlerFunc
	UserAPISetRole       http.HandlerFunc
	UserAPIExportUsers   http.HandlerFunc
	UserAPIExportAudit   http.HandlerFunc
	NotifAPIList         http.HandlerFunc
	NotifAPIUnreadCount  http.HandlerFunc
	NotifAPIMarkRead     http.HandlerFunc
	NotifAPIMarkAllRead  http.HandlerFunc
	NotifAPIDelete       http.HandlerFunc
	DeviceAPIRegister    http.HandlerFunc
	DeviceAPIUnregister  http.HandlerFunc

	// Admin handlers
	AdminListUsers     http.HandlerFunc
	AdminSetEnabled    http.HandlerFunc
	AdminSetRole       http.HandlerFunc
	AdminExportUsers   http.HandlerFunc
	AdminShowAudit     http.HandlerFunc
	AdminExportAudit   http.HandlerFunc
	DevDashboard       http.HandlerFunc
	DevProcessOutbox   http.HandlerFunc
	DevCleanupSessions http.HandlerFunc
	DevInvalidateCache http.HandlerFunc

	// Health/metrics/static
	Health      http.HandlerFunc
	Metrics     http.Handler
	Static      http.Handler
	OpenAPISpec http.HandlerFunc

	// DevMode flag gates the dev dashboard routes.
	DevDashboardEnabled bool
}

// Extension is the core platform extension. It wraps a populated Bundle and
// implements extplatform.Extension by contributing every core route through
// the platform's route registry.
type Extension struct {
	bundle *Bundle
}

// NewExtension creates the core extension from a populated Bundle.
func NewExtension(b Bundle) *Extension {
	return &Extension{bundle: &b}
}

// Manifest declares the core extension's identity, mode, route ownership, and
// migrations. The core extension owns the entire platform surface
// (UI, API, admin, assets) and runs in FullPlatform mode.
func (e *Extension) Manifest() extplatform.Manifest {
	return extplatform.Manifest{
		ID:    "platform-core",
		Label: "Platform Core",
		Mode:  extplatform.FullPlatform,
		Ownership: extplatform.RouteOwnership{
			UI: []string{
				"/", "/auth", "/contacts", "/messages", "/search", "/settings",
				"/notifications", "/components", "/ws",
			},
			API:    []string{"/api/v1", "/metrics", "/openapi.json"},
			Admin:  []string{"/admin", "/dev"},
			Assets: []string{"/static"},
		},
		Migrations: []extplatform.MigrationSet{
			{
				ExtensionID: "platform-core",
				FS:          Migrations,
				Directory:   "migrations",
				Table:       "schema_migrations_core",
			},
		},
	}
}
