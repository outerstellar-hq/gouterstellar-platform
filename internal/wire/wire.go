package wire

import (
	"context"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/platform"
	"github.com/rygel/gouterstellar-platform/internal/platform/core"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/handler"
	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

type App struct {
	Config                *config.Config
	Renderer              *web.Renderer
	MessageService        *service.MessageService
	ContactService        *service.ContactService
	SecurityService       *service.SecurityService
	NotificationService   *service.NotificationService
	OutboxProcessor       *service.OutboxProcessor
	SyncAPI               *handler.SyncAPI
	AuthAPI               *handler.AuthAPI
	AuthHandler           *handler.AuthHandler
	HomeHandler           *handler.HomeHandler
	MessagesHandler       *handler.MessagesHandler
	ContactsHandler       *handler.ContactsHandler
	UserAdminHandler      *handler.UserAdminHandler
	UserAdminAPI          *handler.UserAdminAPI
	NotificationsHandler  *handler.NotificationsHandler
	NotificationAPI       *handler.NotificationAPI
	DeviceRegistrationAPI *handler.DeviceRegistrationAPI
	OAuthHandler          *handler.OAuthHandler
	SearchHandler         *handler.SearchHandler
	SettingsHandler       *handler.SettingsHandler
	ErrorHandler          *handler.ErrorHandler
	DevDashboardHandler   *handler.DevDashboardHandler
	ComponentsHandler     *handler.ComponentsHandler
	OpenAPIHandler        *handler.OpenAPIHandler
	SyncWebSocket         *handler.SyncWebSocket
	Realms                []security.AuthRealm
	Registry              *prometheus.Registry
	EmailService          service.EmailService
	Analytics             service.AnalyticsService
	ActivityUpdater       *security.AsyncActivityUpdater
	JwtService            *security.JwtService
	PermissionResolver    security.PermissionResolver
	ServiceBag            extplatform.ServiceBag
}

func Wire(cfg *config.Config, pool *pgxpool.Pool, templateFS fs.FS) *App {
	registry := prometheus.NewRegistry()

	messageRepo := persistence.NewMessageRepository(pool)
	userRepo := persistence.NewUserRepository(pool)
	sessionRepo := persistence.NewSessionRepository(pool)
	contactRepo := persistence.NewContactRepository(pool)
	notificationRepo := persistence.NewNotificationRepository(pool)
	apiKeyRepo := persistence.NewApiKeyRepository(pool)
	outboxRepo := persistence.NewOutboxRepository(pool)
	auditRepo := persistence.NewAuditRepository(pool)
	deviceTokenRepo := persistence.NewDeviceTokenRepository(pool)
	passwordResetRepo := persistence.NewPasswordResetRepository(pool)
	oauthRepo := persistence.NewOAuthRepository(pool)

	txMgr := persistence.NewTransactionManager(pool)
	messageCache := persistence.NewMessageCache(5 * time.Minute)
	wsPublisher := service.NewWsEventPublisher()

	passwordEncoder := security.NewBCryptPasswordEncoder(12)
	jwtSvc := security.NewJwtService(cfg.JWT)
	activityUpdater := security.NewAsyncActivityUpdater(userRepo)
	permissionResolver := security.NewRoleBasedPermissionResolver()

	securitySvc := service.NewSecurityService(
		userRepo,
		passwordEncoder,
		sessionRepo,
		auditRepo,
		int64(cfg.SessionTimeoutMinutes)*60,
	)

	apiKeySvc := security.NewApiKeyService(apiKeyRepo, userRepo)
	oauthSvc := security.NewOAuthService(userRepo, oauthRepo, passwordEncoder)
	appleProvider := security.NewAppleOAuthProvider()

	sessionRealm := security.NewSessionRealm(func(tokenHash string) model.SessionLookup {
		session, err := sessionRepo.FindByTokenHash(context.Background(), tokenHash)
		if err != nil {
			return model.SessionNotFound{}
		}
		if session.ExpiresAt.Time.Before(time.Now()) {
			return model.SessionExpired{}
		}
		pltUser, err := userRepo.FindByID(context.Background(), session.UserID)
		if err != nil {
			return model.SessionNotFound{}
		}
		user := security.PltUserToModel(pltUser)
		if !user.Enabled {
			return model.SessionNotFound{}
		}
		return model.SessionActive{User: user}
	})

	apiKeyRealm := security.NewApiKeyRealm(func(rawKey string) *model.User {
		user, err := apiKeySvc.AuthenticateApiKey(context.Background(), rawKey)
		if err != nil {
			return nil
		}
		return user
	})

	jwtRealm := security.NewJwtRealm(jwtSvc, func(userID uuid.UUID) *model.User {
		u, err := userRepo.FindByID(context.Background(), userID)
		if err != nil {
			return nil
		}
		user := security.PltUserToModel(u)
		if !user.Enabled {
			return nil
		}
		return user
	})

	realms := []security.AuthRealm{sessionRealm, apiKeyRealm, jwtRealm}

	var emailSvc service.EmailService
	if cfg.Email.Enabled {
		emailSvc = service.NewResilientEmailService(service.NewSmtpEmailService(service.SmtpConfig{
			Host:     cfg.Email.Host,
			Port:     cfg.Email.Port,
			Username: cfg.Email.Username,
			Password: cfg.Email.Password,
			From:     cfg.Email.From,
			StartTLS: cfg.Email.StartTLS,
		}))
	} else if cfg.DevMode {
		emailSvc = &service.ConsoleEmailService{}
	} else {
		emailSvc = &service.NoOpEmailService{}
	}

	analytics := &service.NoOpAnalyticsService{}

	messageSvc := service.NewMessageService(messageRepo, outboxRepo, txMgr, messageCache, wsPublisher, auditRepo)
	contactSvc := service.NewContactService(contactRepo, outboxRepo, wsPublisher)
	notificationSvc := service.NewNotificationService(notificationRepo)
	outboxProcessor := service.NewOutboxProcessor(outboxRepo, txMgr)
	passwordResetSvc := service.NewPasswordResetService(userRepo, passwordEncoder, passwordResetRepo, emailSvc, auditRepo)

	renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap(), cfg.Version)
	if err != nil {
		panic("failed to parse templates: " + err.Error())
	}

	syncAPI := handler.NewSyncAPI(messageSvc, contactSvc, analytics)
	authAPI := handler.NewAuthAPI(securitySvc, apiKeySvc, passwordResetSvc, cfg.SessionCookieSecure, analytics, jwtSvc)
	authHandler := handler.NewAuthHandler(securitySvc, passwordResetSvc, renderer, cfg.SessionCookieSecure, analytics)
	homeHandler := handler.NewHomeHandler(messageSvc, contactSvc, securitySvc, renderer, cfg.Version)
	messagesHandler := handler.NewMessagesHandler(messageSvc, renderer)
	contactsHandler := handler.NewContactsHandler(contactSvc, renderer)
	userAdminHandler := handler.NewUserAdminHandler(securitySvc, renderer)
	userAdminAPI := handler.NewUserAdminAPI(securitySvc)
	notificationsHandler := handler.NewNotificationsHandler(notificationSvc, renderer)
	notificationAPI := handler.NewNotificationAPI(notificationSvc)
	deviceRegistrationAPI := handler.NewDeviceRegistrationAPI(deviceTokenRepo)
	oauthHandler := handler.NewOAuthHandler(securitySvc, oauthSvc, cfg.SessionCookieSecure, appleProvider, cfg.AppBaseURL)
	searchHandler := handler.NewSearchHandler(messageSvc, contactSvc, renderer)
	settingsHandler := handler.NewSettingsHandler(securitySvc, apiKeySvc, renderer)
	errorHandler := handler.NewErrorHandler(renderer, cfg.Version)
	devDashboardHandler := handler.NewDevDashboardHandler(outboxProcessor, securitySvc, messageSvc, renderer, cfg.DevDashboardEnabled)
	componentsHandler := handler.NewComponentsHandler(messageSvc, contactSvc, renderer)
	openAPIHandler := handler.NewOpenAPIHandler()
	syncWebSocket := handler.NewSyncWebSocket(wsPublisher, sessionRepo, userRepo, cfg.SessionCookieSecure)

	svcBag := platform.BuildServiceBag(messageSvc, contactSvc, securitySvc)

	return &App{
		Config:                cfg,
		Renderer:              renderer,
		MessageService:        messageSvc,
		ContactService:        contactSvc,
		SecurityService:       securitySvc,
		NotificationService:   notificationSvc,
		OutboxProcessor:       outboxProcessor,
		SyncAPI:               syncAPI,
		AuthAPI:               authAPI,
		AuthHandler:           authHandler,
		HomeHandler:           homeHandler,
		MessagesHandler:       messagesHandler,
		ContactsHandler:       contactsHandler,
		UserAdminHandler:      userAdminHandler,
		UserAdminAPI:          userAdminAPI,
		NotificationsHandler:  notificationsHandler,
		NotificationAPI:       notificationAPI,
		DeviceRegistrationAPI: deviceRegistrationAPI,
		OAuthHandler:          oauthHandler,
		SearchHandler:         searchHandler,
		SettingsHandler:       settingsHandler,
		ErrorHandler:          errorHandler,
		DevDashboardHandler:   devDashboardHandler,
		ComponentsHandler:     componentsHandler,
		OpenAPIHandler:        openAPIHandler,
		SyncWebSocket:         syncWebSocket,
		Realms:                realms,
		Registry:              registry,
		EmailService:          emailSvc,
		Analytics:             analytics,
		ActivityUpdater:       activityUpdater,
		JwtService:            jwtSvc,
		PermissionResolver:    permissionResolver,
		ServiceBag:            svcBag,
	}
}

// BuildCoreBundle constructs a core.Bundle from the assembled application's
// handlers. Each Bundle field maps to a method on the corresponding handler
// struct; Go auto-promotes the methods to http.HandlerFunc values.
//
// The Health/Metrics/Static fields are intentionally left zero here because
// they depend on the connection pool and static asset directory that the wire
// root does not own. The caller (main) populates them after construction.
func BuildCoreBundle(app *App, cfg *config.Config) core.Bundle {
	return core.Bundle{
		// PublicUI
		AuthShowLogin:       app.AuthHandler.ShowLogin,
		AuthHandleLogin:     app.AuthHandler.HandleLogin,
		AuthHandleRegister:  app.AuthHandler.HandleRegister,
		AuthHandleLogout:    app.AuthHandler.HandleLogout,
		AuthShowChangePwd:   app.AuthHandler.ShowChangePassword,
		AuthHandleChangePwd: app.AuthHandler.HandleChangePassword,
		AuthShowReset:       app.AuthHandler.ShowResetPassword,
		AuthHandleReset:     app.AuthHandler.HandleResetPassword,
		OAuthRedirect:       app.OAuthHandler.Redirect,
		OAuthCallback:       app.OAuthHandler.Callback,
		OAuthCallbackPost:   app.OAuthHandler.CallbackPost,

		// ProtectedUI
		HomeShow:              app.HomeHandler.Show,
		MessagesShow:          app.MessagesHandler.Show,
		ContactsList:          app.ContactsHandler.List,
		ContactsDetail:        app.ContactsHandler.Detail,
		ContactsCreate:        app.ContactsHandler.Create,
		ContactsUpdate:        app.ContactsHandler.Update,
		ContactsDelete:        app.ContactsHandler.Delete,
		SearchSearch:          app.SearchHandler.Search,
		SettingsShow:          app.SettingsHandler.Show,
		SettingsProfile:       app.SettingsHandler.UpdateProfile,
		SettingsPassword:      app.SettingsHandler.ChangePassword,
		SettingsPreferences:   app.SettingsHandler.UpdatePreferences,
		SettingsCreateAPIKey:  app.SettingsHandler.CreateApiKey,
		SettingsDeleteAPIKey:  app.SettingsHandler.DeleteApiKey,
		SettingsNotifPrefs:    app.SettingsHandler.UpdateNotificationPrefs,
		NotifsList:            app.NotificationsHandler.List,
		NotifsMarkRead:        app.NotificationsHandler.MarkRead,
		NotifsMarkAllRead:     app.NotificationsHandler.MarkAllRead,
		NotifsDelete:          app.NotificationsHandler.Delete,
		ComponentsMsgList:     app.ComponentsHandler.MessageList,
		ComponentsContactList: app.ComponentsHandler.ContactList,
		SyncWebSocket:         app.SyncWebSocket.Handle,

		// API
		SyncPullMessages:     app.SyncAPI.PullMessages,
		SyncPushMessages:     app.SyncAPI.PushMessages,
		SyncPullContacts:     app.SyncAPI.PullContacts,
		SyncPushContacts:     app.SyncAPI.PushContacts,
		AuthAPILogin:         app.AuthAPI.Login,
		AuthAPIToken:         app.AuthAPI.IssueToken,
		AuthAPIRegister:      app.AuthAPI.Register,
		AuthAPIChangePwd:     app.AuthAPI.ChangePassword,
		AuthAPIResetReq:      app.AuthAPI.RequestPasswordReset,
		AuthAPIConfirmReset:  app.AuthAPI.ConfirmPasswordReset,
		AuthAPILogout:        app.AuthAPI.Logout,
		AuthAPIGetProfile:    app.AuthAPI.GetProfile,
		AuthAPIUpdateProfile: app.AuthAPI.UpdateProfile,
		AuthAPINotifPrefs:    app.AuthAPI.UpdateNotificationPreferences,
		AuthAPIDeleteAccount: app.AuthAPI.DeleteAccount,
		AuthAPICreateAPIKey:  app.AuthAPI.CreateApiKey,
		AuthAPIListAPIKeys:   app.AuthAPI.ListApiKeys,
		AuthAPIDeleteAPIKey:  app.AuthAPI.DeleteApiKey,
		UserAPIListUsers:     app.UserAdminAPI.ListUsers,
		UserAPICountUsers:    app.UserAdminAPI.CountUsers,
		UserAPISetEnabled:    app.UserAdminAPI.SetEnabled,
		UserAPISetRole:       app.UserAdminAPI.SetRole,
		UserAPIExportUsers:   app.UserAdminAPI.ExportUsersCSV,
		UserAPIExportAudit:   app.UserAdminAPI.ExportAuditCSV,
		NotifAPIList:         app.NotificationAPI.List,
		NotifAPIUnreadCount:  app.NotificationAPI.UnreadCount,
		NotifAPIMarkRead:     app.NotificationAPI.MarkRead,
		NotifAPIMarkAllRead:  app.NotificationAPI.MarkAllRead,
		NotifAPIDelete:       app.NotificationAPI.Delete,
		DeviceAPIRegister:    app.DeviceRegistrationAPI.Register,
		DeviceAPIUnregister:  app.DeviceRegistrationAPI.Unregister,

		// Admin
		AdminListUsers:     app.UserAdminHandler.ListUsers,
		AdminSetEnabled:    app.UserAdminHandler.SetEnabled,
		AdminSetRole:       app.UserAdminHandler.SetRole,
		AdminExportUsers:   app.UserAdminHandler.ExportUsers,
		AdminShowAudit:     app.UserAdminHandler.ShowAudit,
		AdminExportAudit:   app.UserAdminHandler.ExportAudit,
		DevDashboard:       app.DevDashboardHandler.Show,
		DevProcessOutbox:   app.DevDashboardHandler.ProcessOutbox,
		DevCleanupSessions: app.DevDashboardHandler.CleanupSessions,
		DevInvalidateCache: app.DevDashboardHandler.InvalidateCache,

		// API docs
		OpenAPISpec: app.OpenAPIHandler.Spec,

		DevDashboardEnabled: cfg.DevDashboardEnabled,
	}
}
