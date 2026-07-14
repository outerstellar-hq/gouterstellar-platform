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
	"github.com/rygel/gouterstellar-platform/internal/web/filter"
	"github.com/rygel/gouterstellar-platform/internal/web/handler"
	"github.com/rygel/gouterstellar-platform/pkg/i18n"
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
	AuthMetrics           *filter.AuthMetrics
	EmailService          service.EmailService
	Analytics             service.AnalyticsService
	I18n                  *i18n.I18nService
	ActivityUpdater       *security.AsyncActivityUpdater
	JwtService            *security.JwtService
	PermissionResolver    security.PermissionResolver
	ServiceBag            extplatform.ServiceBag
}

func Wire(cfg *config.Config, pool *pgxpool.Pool, templateFS fs.FS) *App {
	registry := prometheus.NewRegistry()
	authMetrics := filter.NewAuthMetrics(registry)

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

	notificationSvc := service.NewNotificationService(notificationRepo)

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

	securitySvc := service.NewSecurityService(
		userRepo,
		passwordEncoder,
		sessionRepo,
		auditRepo,
		notificationSvc,
		emailSvc,
		int64(cfg.SessionTimeoutMinutes)*60,
	)

	apiKeySvc := security.NewApiKeyService(apiKeyRepo, userRepo)
	oauthSvc := security.NewOAuthService(userRepo, oauthRepo, passwordEncoder)
	appleProvider := security.NewAppleOAuthProvider()

	// Register the Google OAuth provider when fully configured; otherwise leave
	// it nil so resolveProvider treats "google" as unsupported.
	var googleProvider *security.GoogleOAuthProvider
	if cfg.OAuth.Google.ClientID != "" && cfg.OAuth.Google.ClientSecret != "" {
		googleProvider = security.NewGoogleOAuthProvider(
			cfg.OAuth.Google.ClientID,
			cfg.OAuth.Google.ClientSecret,
			cfg.OAuth.Google.RedirectURI,
		)
	}

	sessionRealm := security.NewSessionRealm(func(ctx context.Context, tokenHash string) model.SessionLookup {
		session, err := sessionRepo.FindByTokenHash(ctx, tokenHash)
		if err != nil {
			return model.SessionNotFound{}
		}
		if session.ExpiresAt.Time.Before(time.Now()) {
			return model.SessionExpired{}
		}
		pltUser, err := userRepo.FindByID(ctx, session.UserID)
		if err != nil {
			return model.SessionNotFound{}
		}
		user := security.PltUserToModel(pltUser)
		if !user.Enabled {
			return model.SessionNotFound{}
		}
		return model.SessionActive{User: user}
	})

	apiKeyRealm := security.NewApiKeyRealm(func(ctx context.Context, rawKey string) *model.User {
		user, err := apiKeySvc.AuthenticateApiKey(ctx, rawKey)
		if err != nil {
			return nil
		}
		return user
	})

	jwtRealm := security.NewJwtRealm(jwtSvc, func(ctx context.Context, userID uuid.UUID) *model.User {
		u, err := userRepo.FindByID(ctx, userID)
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

	var analytics service.AnalyticsService
	if cfg.Segment.Enabled && cfg.Segment.WriteKey != "" {
		analytics = service.NewSegmentAnalyticsService(cfg.Segment.WriteKey)
	} else {
		analytics = &service.NoOpAnalyticsService{}
	}

	messageSvc := service.NewMessageService(messageRepo, outboxRepo, txMgr, messageCache, wsPublisher, auditRepo, notificationSvc, emailSvc)
	contactSvc := service.NewContactService(contactRepo, outboxRepo, txMgr, wsPublisher, notificationSvc)
	outboxProcessor := service.NewOutboxProcessor(outboxRepo, txMgr, wsPublisher)
	passwordResetSvc := service.NewPasswordResetService(userRepo, passwordEncoder, passwordResetRepo, emailSvc, auditRepo, cfg.AppBaseURL)

	// Instantiate the i18n service from the embedded locale bundles and publish
	// it to the web package so the translate template func can resolve keys at
	// render time. Installed before the renderer is constructed so the func map
	// closure captures a usable service.
	i18nSvc := i18n.NewI18nService(web.LocaleFS, web.LocaleBasePath)
	web.SetGlobalI18nService(i18nSvc)

	renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap(), cfg.Version)
	if err != nil {
		panic("failed to parse templates: " + err.Error())
	}

	syncAPI := handler.NewSyncAPI(messageSvc, contactSvc, analytics)
	authAPI := handler.NewAuthAPI(securitySvc, apiKeySvc, passwordResetSvc, cfg.SessionCookieSecure, analytics, jwtSvc)
	authHandler := handler.NewAuthHandler(securitySvc, passwordResetSvc, renderer, cfg.SessionCookieSecure, analytics, googleProvider != nil)
	homeHandler := handler.NewHomeHandler(messageSvc, contactSvc, securitySvc, renderer, cfg.Version)
	messagesHandler := handler.NewMessagesHandler(messageSvc, renderer)
	contactsHandler := handler.NewContactsHandler(contactSvc, renderer)
	userAdminHandler := handler.NewUserAdminHandler(securitySvc, renderer)
	userAdminAPI := handler.NewUserAdminAPI(securitySvc)
	notificationsHandler := handler.NewNotificationsHandler(notificationSvc, renderer)
	notificationAPI := handler.NewNotificationAPI(notificationSvc)
	deviceRegistrationAPI := handler.NewDeviceRegistrationAPI(deviceTokenRepo)

	// Coerce the optional Google provider into the interface without producing
	// a non-nil interface wrapping a nil pointer: a bare nil assignment yields
	// a nil interface, which resolveProvider treats as "unsupported".
	var googleProviderIfc security.OAuthProvider
	if googleProvider != nil {
		googleProviderIfc = googleProvider
	}
	oauthHandler := handler.NewOAuthHandler(securitySvc, oauthSvc, cfg.SessionCookieSecure, appleProvider, googleProviderIfc, cfg.AppBaseURL)
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
		AuthMetrics:           authMetrics,
		EmailService:          emailSvc,
		Analytics:             analytics,
		I18n:                  i18nSvc,
		ActivityUpdater:       activityUpdater,
		JwtService:            jwtSvc,
		PermissionResolver:    permissionResolver,
		ServiceBag:            svcBag,
	}
}

// BuildCoreExtension constructs the core platform Extension from the assembled
// application's handlers. Each handler is a RouteContributor that registers its
// own routes via the contribution context, so the only wiring here is the
// ordered list of contributors plus the OpenAPI spec handler (health/metrics/
// static are infrastructure routes populated by the caller after construction).
func BuildCoreExtension(app *App) *core.Extension {
	ext := core.NewExtension()
	ext.SetOpenAPI(app.OpenAPIHandler.Spec)
	ext.AddContributors(
		app.AuthHandler,
		app.OAuthHandler,
		app.AuthAPI,
		app.SyncAPI,
		app.NotificationAPI,
		app.UserAdminAPI,
		app.DeviceRegistrationAPI,
		app.HomeHandler,
		app.MessagesHandler,
		app.ContactsHandler,
		app.SearchHandler,
		app.SettingsHandler,
		app.NotificationsHandler,
		app.ComponentsHandler,
		app.SyncWebSocket,
		app.UserAdminHandler,
		app.DevDashboardHandler,
	)
	return ext
}
