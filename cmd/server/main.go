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

	"github.com/rygel/gouterstellar-platform/extensions/reports"
	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/platform/core"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/filter"
	"github.com/rygel/gouterstellar-platform/internal/wire"
	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

func main() {
	cfg := config.Load()
	slog.Info("Starting Outerstellar Platform", "version", cfg.Version, "port", cfg.Port)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
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

	// Build the core extension bundle from the wired application handlers,
	// then populate the health/metrics/static handlers that depend on the
	// connection pool and filesystem.
	coreBundle := wire.BuildCoreBundle(app, cfg)
	coreBundle.Health = healthHandler(pool)
	coreBundle.Metrics = promhttp.HandlerFor(app.Registry, promhttp.HandlerOpts{})
	coreBundle.Static = http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	coreExt := core.NewExtension(coreBundle)

	reportsExt := reports.New(app.ServiceBag.MessageCounter)

	// The middleware chain is applied to every route in the same order as
	// the previous Chi-based wire root.
	middlewareChain := []func(http.Handler) http.Handler{
		chimw.RequestID,
		chimw.RealIP,
		chimw.Recoverer,
		chimw.Timeout(60 * time.Second),
		cors.Handler(cors.Options{
			AllowedOrigins:   strings.Split(cfg.CORSOrigins, ","),
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}),
		filter.SecurityHeaders(cfg.CSPPolicy, cfg.SessionCookieSecure),
		filter.DevAutoLogin(func() uuid.UUID {
			return app.SecurityService.DevAdminID(ctx)
		}, app.SecurityService, cfg.DevMode),
		filter.RateLimiter(10, 20),
		filter.CSRF(cfg.CSRFEnabled),
		filter.Session(app.SecurityService, cfg.SessionCookieSecure),
		filter.Logging(),
	}

	handler, err := extplatform.NewHandler(extplatform.Options{
		Mode: extplatform.FullPlatform,
		Extensions: []extplatform.Extension{
			coreExt,
			reportsExt,
		},
		Services:        app.ServiceBag,
		MiddlewareChain: middlewareChain,
		GroupMiddleware: map[extplatform.RouteGroup][]func(http.Handler) http.Handler{
			// Bearer token auth (API key / JWT) for JSON API routes.
			extplatform.GroupAPI: {filter.BearerAuth(app.Realms...)},
		},
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

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := app.OutboxProcessor.ProcessPending(context.Background()); err != nil {
				slog.Error("Outbox processing failed", "error", err)
			}
			app.ActivityUpdater.Flush()
			if err := app.SecurityService.DeleteExpiredSessions(context.Background()); err != nil {
				slog.Error("Session cleanup failed", "error", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	fmt.Println("Server stopped")
}

// healthHandler returns a handler that pings the database to report liveness.
func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"unhealthy","database":"down","error":%q}`, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"healthy","database":"up"}`)
	}
}
