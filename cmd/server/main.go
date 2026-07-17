package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/outerstellar-hq/gouterstellar-platform/extensions/reports"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/config"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/observability"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web/filter"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/wire"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
	"github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("Configuration validation failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Starting Outerstellar Platform", "version", cfg.Version, "build", buildinfo.Current().Identity, "port", cfg.Port)

	ctx := context.Background()

	// Initialise OpenTelemetry tracing as early as possible so spans are
	// captured for every subsequent operation. The shutdown function flushes
	// pending spans on exit.
	tracingShutdown, err := observability.SetupTracing(ctx, "outerstellar-platform")
	if err != nil {
		slog.Error("Failed to initialise tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracingShutdown(shutdownCtx); err != nil {
			slog.Error("Tracing shutdown error", "error", err)
		}
	}()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to parse database config", "error", err)
		os.Exit(1)
	}
	poolConfig.ConnConfig.Tracer = persistence.NewTracingTracer()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("Database ping failed", "error", err)
		os.Exit(1)
	}

	templateFS := web.TemplateFS()
	app := wire.Wire(cfg, pool, templateFS)

	// Build the core extension from the wired application handlers, then
	// populate the health/metrics/static handlers that depend on the
	// connection pool and filesystem.
	catalog := extplatform.NewCatalog()
	coreExt := wire.BuildCoreExtension(app, catalog)
	coreExt.SetOperations(
		localhostOnly(livenessHandler()),
		localhostOnly(readinessHandler(pool.Ping)),
		robotsHandler(),
		sitemapHandler(cfg.AppBaseURL),
	)
	coreExt.SetMetrics(promhttp.HandlerFor(app.Registry, promhttp.HandlerOpts{}))
	coreExt.SetDiagnostics(localhostOnly(routeDiagnosticsHandler(catalog)))
	coreExt.SetStatic(os.DirFS("static"))

	reportsExt := reports.New(app.ServiceBag.MessageCounter)

	// The middleware chain is applied to every route in the same order as
	// the previous Chi-based wire root.
	middlewareChain := []func(http.Handler) http.Handler{
		// otelhttp is first so it wraps the entire chain, creating a root
		// span for every request regardless of which downstream middleware
		// short-circuits.
		otelhttp.NewMiddleware("outerstellar-platform"),
		filter.Logging(),
		filter.ErrorHandler(app.ErrorHandler.InternalError),
		filter.Metrics(app.Registry),
		chimw.Timeout(60 * time.Second),
		cors.Handler(cors.Options{
			AllowedOrigins:   strings.Split(cfg.CORSOrigins, ","),
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", filter.RequestIDHeader},
			ExposedHeaders:   []string{"Link", filter.RequestIDHeader, filter.SessionExpiredHeader},
			AllowCredentials: true,
			MaxAge:           300,
		}),
		filter.MaxBodySize(cfg.MaxRequestBodyBytes),
		filter.SecurityHeaders(cfg.CSPPolicy, cfg.SessionCookieSecure),
		filter.DevAutoLogin(func() uuid.UUID {
			return app.SecurityService.DevAdminID(ctx)
		}, app.SecurityService, cfg.DevMode),
		filter.AuthRateLimiter(10, time.Minute, strings.Split(cfg.TrustedProxies, ",")),
		filter.CSRF(cfg.CSRFEnabled, cfg.SessionCookieSecure),
		filter.Session(app.SecurityService, cfg.SessionCookieSecure),
	}

	mode := extplatform.PlatformMode(cfg.PlatformMode)
	if mode == "" {
		mode = extplatform.FullPlatform
	}

	handler, err := extplatform.NewHandler(extplatform.Options{
		Mode:            mode,
		StaticDir:       cfg.StaticDir,
		AssetMiddleware: filter.ETag(),
		Extensions: []extplatform.Extension{
			coreExt,
			reportsExt,
		},
		Services:        app.ServiceBag,
		MiddlewareChain: middlewareChain,
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			// Browser application routes require a valid session, but no role-specific
			// permission beyond authentication.
			extplatform.GroupProtectedUI: {filter.RequireAuthenticated},
			// Bearer token auth (API key / JWT) for JSON API routes.
			extplatform.GroupAPI: {filter.BearerAuth(app.AuthMetrics, app.Realms...)},
			// Admin routes require the wildcard admin permission.
			extplatform.GroupAdmin: {filter.RequirePermission(app.PermissionResolver, "*", "*")},
		},
		NotFoundHandler: http.HandlerFunc(app.ErrorHandler.NotFound),
		Catalog:         catalog,
	})
	if err != nil {
		slog.Error("Platform assembly failed", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// workerCtx is cancelled when shutdown begins so that background workers
	// (outbox processor, session cleanup) stop promptly instead of running
	// until process exit.
	workerCtx, workerCancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := app.OutboxProcessor.ProcessPending(workerCtx); err != nil {
					slog.Error("Outbox processing failed", "error", err)
				}
				if err := app.SecurityService.DeleteExpiredSessions(workerCtx); err != nil {
					slog.Error("Session cleanup failed", "error", err)
				}
			case <-workerCtx.Done():
				return
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down...")
	// Stop background workers first, before tearing down the HTTP server.
	workerCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	fmt.Println("Server stopped")
}
