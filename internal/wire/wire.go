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
	"github.com/rygel/gouterstellar-platform/internal/platform/core"
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/filter"
	"github.com/rygel/gouterstellar-platform/internal/web/handler"
	"github.com/rygel/gouterstellar-platform/pkg/i18n"
	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

// repos groups every repository instance assembled from the connection pool.
// Keeping them in one struct lets buildServices and buildApp depend on a single
// value rather than eleven loose parameters.
type repos struct {
	messageRepo       persistence.MessageRepository
	userRepo          persistence.UserRepository
	sessionRepo       persistence.SessionRepository
	contactRepo       persistence.ContactRepository
	notificationRepo  persistence.NotificationRepository
	apiKeyRepo        persistence.ApiKeyRepository
	outboxRepo        persistence.OutboxRepository
	auditRepo         persistence.AuditRepository
	deviceTokenRepo   persistence.DeviceTokenRepository
	passwordResetRepo persistence.PasswordResetRepository
	oauthRepo         persistence.OAuthRepository
	totpRepo          persistence.TOTPRepository
}

// buildRepos constructs all repository implementations from the pool. The sync
// repos (message/contact) also need the outbox repo at the service layer, but
// at the repo layer each is independent.
func buildRepos(pool *pgxpool.Pool) repos {
	return repos{
		messageRepo:       persistence.NewMessageRepository(pool),
		userRepo:          persistence.NewUserRepository(pool),
		sessionRepo:       persistence.NewSessionRepository(pool),
		contactRepo:       persistence.NewContactRepository(pool),
		notificationRepo:  persistence.NewNotificationRepository(pool),
		apiKeyRepo:        persistence.NewApiKeyRepository(pool),
		outboxRepo:        persistence.NewOutboxRepository(pool),
		auditRepo:         persistence.NewAuditRepository(pool),
		deviceTokenRepo:   persistence.NewDeviceTokenRepository(pool),
		passwordResetRepo: persistence.NewPasswordResetRepository(pool),
		oauthRepo:         persistence.NewOAuthRepository(pool),
		totpRepo:          persistence.NewTOTPRepository(pool),
	}
}

// services groups every service and shared security/infra dependency built from
// the config and repos. Handler construction (buildApp) reads from this struct
// so the handler layer never re-derives services itself.
type services struct {
	messageSvc         *service.MessageService
	contactSvc         *service.ContactService
	securitySvc        *service.SecurityService
	totpSvc            *service.TOTPService
	notificationSvc    *service.NotificationService
	outboxProcessor    *service.OutboxProcessor
	passwordResetSvc   *service.PasswordResetService
	apiKeySvc          *security.ApiKeyService
	oauthSvc           *security.OAuthService
	googleProvider     *security.GoogleOAuthProvider
	realms             []security.AuthRealm
	emailSvc           service.EmailService
	analytics          service.AnalyticsService
	jwtSvc             *security.JwtService
	permissionResolver security.PermissionResolver
	wsPublisher        *service.WsEventPublisher
}

// buildServices constructs the service layer plus the shared cross-cutting
// dependencies (realms, email, analytics, JWT, activity tracking) that handlers
// and the App struct both consume. The realm closures live here because they
// bind auth logic to the session/api-key/jwt lookups; moving them into the
// security package would require interface changes and is deferred.
func buildServices(cfg *config.Config, r repos, pool *pgxpool.Pool) (*services, error) {
	txMgr := persistence.NewTransactionManager(pool)
	messageCache := persistence.NewMessageCache(5 * time.Minute)
	wsPublisher := service.NewWsEventPublisher()

	passwordEncoder := security.NewBCryptPasswordEncoder(12)
	jwtSvc := security.NewJwtService(cfg.JWT)
	permissionResolver := security.NewRoleBasedPermissionResolver()

	notificationSvc := service.NewNotificationService(r.notificationRepo)

	emailSvc, err := buildEmailService(cfg)
	if err != nil {
		return nil, err
	}

	securityConfig := service.SecurityConfig{
		SessionTimeout:         time.Duration(cfg.SessionTimeoutMinutes) * time.Minute,
		SessionAbsoluteTimeout: time.Duration(cfg.SessionAbsoluteMinutes) * time.Minute,
		MaxFailedLoginAttempts: cfg.MaxFailedLoginAttempts,
		LockoutDuration:        time.Duration(cfg.LockoutDurationSeconds) * time.Second,
		RegistrationEnabled:    cfg.RegistrationEnabled,
	}
	totpSvc := service.NewTOTPService(r.totpRepo, r.userRepo, passwordEncoder, service.NewAuditService(r.auditRepo), securityConfig)
	securitySvc := service.NewSecurityService(
		service.SecurityDependencies{
			UserRepository:      r.userRepo,
			PasswordEncoder:     passwordEncoder,
			SessionRepository:   r.sessionRepo,
			AuditRepository:     r.auditRepo,
			NotificationService: notificationSvc,
			EmailService:        emailSvc,
			TOTPService:         totpSvc,
		},
		securityConfig,
	)

	apiKeySvc := security.NewApiKeyService(r.apiKeyRepo, r.userRepo)
	oauthSvc := security.NewOAuthService(r.userRepo, r.oauthRepo, passwordEncoder)

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

	realms := buildRealms(r, securitySvc, jwtSvc, apiKeySvc)

	var analytics service.AnalyticsService
	if cfg.Segment.Enabled && cfg.Segment.WriteKey != "" {
		analytics = service.NewSegmentAnalyticsService(cfg.Segment.WriteKey)
	} else {
		analytics = &service.NoOpAnalyticsService{}
	}

	messageSvc := service.NewMessageService(r.messageRepo, r.outboxRepo, txMgr, messageCache, wsPublisher, notificationSvc, emailSvc)
	contactSvc := service.NewContactService(r.contactRepo, r.outboxRepo, txMgr, wsPublisher, notificationSvc)
	outboxProcessor := service.NewOutboxProcessor(r.outboxRepo, txMgr, wsPublisher)
	passwordResetSvc := service.NewPasswordResetService(r.userRepo, passwordEncoder, r.passwordResetRepo, emailSvc, service.NewAuditService(r.auditRepo), cfg.AppBaseURL)

	return &services{
		messageSvc:         messageSvc,
		contactSvc:         contactSvc,
		securitySvc:        securitySvc,
		totpSvc:            totpSvc,
		notificationSvc:    notificationSvc,
		outboxProcessor:    outboxProcessor,
		passwordResetSvc:   passwordResetSvc,
		apiKeySvc:          apiKeySvc,
		oauthSvc:           oauthSvc,
		googleProvider:     googleProvider,
		realms:             realms,
		emailSvc:           emailSvc,
		analytics:          analytics,
		jwtSvc:             jwtSvc,
		permissionResolver: permissionResolver,
		wsPublisher:        wsPublisher,
	}, nil
}

// buildEmailService selects the email backend from config: SMTP when enabled,
// the console sink in dev mode, otherwise a no-op so the app never holds a nil
// EmailService.
func buildEmailService(cfg *config.Config) (service.EmailService, error) {
	if cfg.Email.Enabled {
		return service.NewResilientEmailService(service.NewSmtpEmailService(service.SmtpConfig{
			Host:     cfg.Email.Host,
			Port:     cfg.Email.Port,
			Username: cfg.Email.Username,
			Password: cfg.Email.Password,
			From:     cfg.Email.From,
			StartTLS: cfg.Email.StartTLS,
		})), nil
	}
	if cfg.DevMode {
		return &service.ConsoleEmailService{}, nil
	}
	return &service.NoOpEmailService{}, nil
}

// buildRealms assembles session, API-key, and JWT authentication. Session
// tokens use the same service policy as cookie authentication.
func buildRealms(
	r repos,
	securitySvc *service.SecurityService,
	jwtSvc *security.JwtService,
	apiKeySvc *security.ApiKeyService,
) []security.AuthRealm {
	sessionRealm := security.NewSessionRealm(securitySvc.LookupSession)

	apiKeyRealm := security.NewApiKeyRealm(func(ctx context.Context, rawKey string) *model.User {
		user, err := apiKeySvc.AuthenticateApiKey(ctx, rawKey)
		if err != nil {
			return nil
		}
		return user
	})

	jwtRealm := security.NewJwtRealm(jwtSvc, func(ctx context.Context, userID uuid.UUID) *model.User {
		u, err := r.userRepo.FindByID(ctx, userID)
		if err != nil {
			return nil
		}
		user := security.PltUserToModel(u)
		if !user.Enabled {
			return nil
		}
		return user
	})

	return []security.AuthRealm{sessionRealm, apiKeyRealm, jwtRealm}
}

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
	DataExportHandler     *handler.DataExportHandler
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
	JwtService            *security.JwtService
	PermissionResolver    security.PermissionResolver
	ServiceBag            extplatform.ServiceBag
}

func Wire(cfg *config.Config, pool *pgxpool.Pool, templateFS fs.FS) *App {
	registry := prometheus.NewRegistry()
	authMetrics := filter.NewAuthMetrics(registry)

	repos := buildRepos(pool)
	svcs, err := buildServices(cfg, repos, pool)
	if err != nil {
		panic(err)
	}
	return buildApp(cfg, repos, svcs, templateFS, registry, authMetrics)
}

// buildApp constructs the handler layer and assembles the final App. It owns
// the renderer, i18n service, and all HTTP handlers; everything else is read
// from the assembled repos and services.
func buildApp(cfg *config.Config, r repos, svcs *services, templateFS fs.FS, registry *prometheus.Registry, authMetrics *filter.AuthMetrics) *App {
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

	syncAPI := handler.NewSyncAPI(svcs.messageSvc, svcs.contactSvc, svcs.analytics)
	authAPI := handler.NewAuthAPI(svcs.securitySvc, svcs.totpSvc, svcs.apiKeySvc, svcs.passwordResetSvc, cfg.SessionCookieSecure, svcs.analytics, svcs.jwtSvc)
	authHandler := handler.NewAuthHandler(svcs.securitySvc, svcs.totpSvc, svcs.passwordResetSvc, renderer, cfg.SessionCookieSecure, svcs.analytics, svcs.googleProvider != nil)
	homeHandler := handler.NewHomeHandler(svcs.messageSvc, svcs.contactSvc, svcs.securitySvc, renderer, cfg.Version)
	messagesHandler := handler.NewMessagesHandler(svcs.messageSvc, renderer)
	contactsHandler := handler.NewContactsHandler(svcs.contactSvc, renderer)
	userAdminHandler := handler.NewUserAdminHandler(svcs.securitySvc, renderer)
	userAdminAPI := handler.NewUserAdminAPI(svcs.securitySvc)
	dataExportHandler := handler.NewDataExportHandler(svcs.messageSvc, svcs.contactSvc)
	notificationsHandler := handler.NewNotificationsHandler(svcs.notificationSvc, renderer)
	notificationAPI := handler.NewNotificationAPI(svcs.notificationSvc)
	deviceRegistrationAPI := handler.NewDeviceRegistrationAPI(r.deviceTokenRepo)

	// Coerce the optional Google provider into the interface without producing
	// a non-nil interface wrapping a nil pointer: a bare nil assignment yields
	// a nil interface, which resolveProvider treats as "unsupported".
	var googleProviderIfc security.OAuthProvider
	if svcs.googleProvider != nil {
		googleProviderIfc = svcs.googleProvider
	}
	// Apple OAuth is intentionally not wired: the provider is an unimplemented
	// stub that can only return errors. resolveProvider returns nil for "apple",
	// which surfaces as "Unknown OAuth provider" — preferable to a half-working
	// flow that fails at code exchange.
	oauthHandler := handler.NewOAuthHandler(svcs.securitySvc, svcs.oauthSvc, cfg.SessionCookieSecure, nil, googleProviderIfc, cfg.AppBaseURL)
	searchHandler := handler.NewSearchHandler(svcs.messageSvc, svcs.contactSvc, renderer)
	settingsHandler := handler.NewSettingsHandler(svcs.securitySvc, svcs.totpSvc, svcs.apiKeySvc, renderer)
	errorHandler := handler.NewErrorHandler(renderer, cfg.Version)
	devDashboardHandler := handler.NewDevDashboardHandler(svcs.outboxProcessor, svcs.securitySvc, svcs.messageSvc, renderer, cfg.DevDashboardEnabled)
	componentsHandler := handler.NewComponentsHandler(svcs.messageSvc, svcs.contactSvc, renderer)
	openAPIHandler := handler.NewOpenAPIHandler()
	syncWebSocket := handler.NewSyncWebSocket(svcs.wsPublisher, r.sessionRepo, r.userRepo, cfg.SessionCookieSecure)

	svcBag := extplatform.ServiceBag{
		MessageCounter: messageCounterAdapter{svc: svcs.messageSvc},
	}

	return &App{
		Config:                cfg,
		Renderer:              renderer,
		MessageService:        svcs.messageSvc,
		ContactService:        svcs.contactSvc,
		SecurityService:       svcs.securitySvc,
		NotificationService:   svcs.notificationSvc,
		OutboxProcessor:       svcs.outboxProcessor,
		SyncAPI:               syncAPI,
		AuthAPI:               authAPI,
		AuthHandler:           authHandler,
		HomeHandler:           homeHandler,
		MessagesHandler:       messagesHandler,
		ContactsHandler:       contactsHandler,
		UserAdminHandler:      userAdminHandler,
		UserAdminAPI:          userAdminAPI,
		DataExportHandler:     dataExportHandler,
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
		Realms:                svcs.realms,
		Registry:              registry,
		AuthMetrics:           authMetrics,
		EmailService:          svcs.emailSvc,
		Analytics:             svcs.analytics,
		I18n:                  i18nSvc,
		JwtService:            svcs.jwtSvc,
		PermissionResolver:    svcs.permissionResolver,
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
		app.DataExportHandler,
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

// messageCounterAdapter wraps *service.MessageService as a platform.MessageCounter
// so extensions can read message counts without depending on internal types.
type messageCounterAdapter struct {
	svc *service.MessageService
}

func (a messageCounterAdapter) CountMessages(ctx context.Context) (int64, error) {
	return a.svc.CountMessages(ctx)
}
