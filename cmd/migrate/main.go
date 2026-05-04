package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if len(os.Args) > 1 {
		dbURL = os.Args[1]
	}
	if dbURL == "" {
		slog.Error("DATABASE_URL env var or CLI argument required")
		os.Exit(1)
	}

	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		slog.Error("parse config failed", "error", err)
		os.Exit(1)
	}
	config.ConnConfig.Password = os.Getenv("DATABASE_PASSWORD")
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	files, err := os.ReadDir("migrations")
	if err != nil {
		slog.Error("read migrations dir", "error", err)
		os.Exit(1)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		slog.Info("Applying migration", "file", f.Name())
		content, err := os.ReadFile("migrations/" + f.Name())
		if err != nil {
			slog.Error("read file", "error", err)
			os.Exit(1)
		}
		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			slog.Error("migration failed", "file", f.Name(), "error", err)
			os.Exit(1)
		}
		slog.Info("Applied", "file", f.Name())
	}

	fmt.Println("All migrations applied successfully.")
}
