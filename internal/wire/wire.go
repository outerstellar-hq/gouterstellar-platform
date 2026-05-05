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
	"github.com/rygel/gouterstellar-platform/internal/security"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/handler"
	"github.com/rygel/gouterstellar-platform/pkg/plugin"
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
	SyncWebSocket         *handler.SyncWebSocket
	Realms                []security.AuthRealm
	Registry              *prometheus.Registry
	EmailService          service.EmailService
	Analytics             service.AnalyticsService
	ActivityUpdater       *security.AsyncActivityUpdater
	JwtService            *security.JwtService
	PermissionResolver    security.PermissionResolver
	PluginManager         *plugin.PluginManager
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

	renderer, err := web.NewRenderer(templateFS, web.TemplateFuncMap())
	if err != nil {
		panic("failed to parse templates: " + err.Error())
	}

	syncAPI := handler.NewSyncAPI(messageSvc, contactSvc, analytics)
	authAPI := handler.NewAuthAPI(securitySvc, apiKeySvc, passwordResetSvc, cfg.SessionCookieSecure, analytics, jwtSvc)
	authHandler := handler.NewAuthHandler(securitySvc, passwordResetSvc, renderer, cfg.SessionCookieSecure, analytics)
	homeHandler := handler.NewHomeHandler(messageSvc, contactSvc, securitySvc, renderer, cfg.Version)
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
	syncWebSocket := handler.NewSyncWebSocket(wsPublisher, sessionRepo, userRepo, cfg.SessionCookieSecure)
	pluginManager := plugin.NewPluginManager()

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
		SyncWebSocket:         syncWebSocket,
		Realms:                realms,
		Registry:              registry,
		EmailService:          emailSvc,
		Analytics:             analytics,
		ActivityUpdater:       activityUpdater,
		JwtService:            jwtSvc,
		PermissionResolver:    permissionResolver,
		PluginManager:         pluginManager,
	}
}
