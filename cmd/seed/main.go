package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/rygel/gouterstellar-platform/internal/config"
)

func main() {
	defaultUsername := os.Getenv("ADMIN_USERNAME")
	if defaultUsername == "" {
		defaultUsername = "admin"
	}
	adminUsername := flag.String("username", defaultUsername, "Admin username (or ADMIN_USERNAME)")
	adminPassword := flag.String("password", os.Getenv("ADMIN_PASSWORD"), "Admin password (or ADMIN_PASSWORD)")
	flag.Parse()
	if *adminPassword == "" {
		slog.Error("Admin password is required", "hint", "set ADMIN_PASSWORD or pass -password")
		os.Exit(2)
	}

	cfg := config.Load()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM plt_users WHERE username = $1", *adminUsername).Scan(&count); err != nil {
		slog.Error("Failed to check existing user", "error", err)
		os.Exit(1)
	}
	if count > 0 {
		slog.Info("Admin user already exists", "username", *adminUsername)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*adminPassword), 12)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		os.Exit(1)
	}

	adminID := uuid.New()
	_, err = pool.Exec(
		ctx,
		`INSERT INTO plt_users (id, username, email, password_hash, role, enabled, email_notifications_enabled, push_notifications_enabled)
		 VALUES ($1, $2, $3, $4, 'ADMIN', true, true, true)`,
		adminID, *adminUsername, fmt.Sprintf("%s@outerstellar.local", *adminUsername), string(hash),
	)
	if err != nil {
		slog.Error("Failed to create admin user", "error", err)
		os.Exit(1)
	}

	slog.Info("Admin user created", "id", adminID, "username", *adminUsername)
}
