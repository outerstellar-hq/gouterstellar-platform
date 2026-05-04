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

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rygel/gouterstellar-platform/internal/config"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/filter"
	"github.com/rygel/gouterstellar-platform/internal/wire"
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

	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   strings.Split(cfg.CORSOrigins, ","),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(filter.SecurityHeaders(cfg.CSPPolicy, cfg.SessionCookieSecure))
	r.Use(filter.RateLimiter(10, 20))
	r.Use(filter.CSRF(cfg.CSRFEnabled))
	r.Use(filter.Session(app.SecurityService, cfg.SessionCookieSecure))
	r.Use(filter.Logging())

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/metrics", promhttp.HandlerFor(app.Registry, promhttp.HandlerOpts{}))

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Route("/", func(r chi.Router) {
		r.Use(filter.BearerAuth(app.Realms...))
		app.SyncAPI.RegisterRoutes(r)
		app.AuthAPI.RegisterRoutes(r)
		app.UserAdminAPI.RegisterRoutes(r)
		app.NotificationAPI.RegisterRoutes(r)
		app.DeviceRegistrationAPI.RegisterRoutes(r)
	})

	app.AuthHandler.RegisterRoutes(r)
	app.HomeHandler.RegisterRoutes(r)
	app.ContactsHandler.RegisterRoutes(r)
	app.SearchHandler.RegisterRoutes(r)
	app.SettingsHandler.RegisterRoutes(r)
	app.OAuthHandler.RegisterRoutes(r)
	app.NotificationsHandler.RegisterRoutes(r)
	app.ComponentsHandler.RegisterRoutes(r)

	r.Route("/admin", func(r chi.Router) {
		app.UserAdminHandler.RegisterRoutes(r)
		if cfg.DevDashboardEnabled {
			app.DevDashboardHandler.RegisterRoutes(r)
		}
	})

	r.NotFound(app.ErrorHandler.NotFound)
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("405 method not allowed"))
	})

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      r,
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
