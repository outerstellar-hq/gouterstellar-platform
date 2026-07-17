package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
	"github.com/outerstellar-hq/gouterstellar-platform/platform/migration"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/platform/core"
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
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Build the migration sets from the same extension manifests the server uses.
	// core.NewExtension() returns an empty extension: Manifest() only returns
	// static data (it never touches the contributors), so this is safe.
	coreExt := core.NewExtension()
	sets := []extplatform.MigrationSet{}
	sets = append(sets, coreExt.Manifest().Migrations...)

	// NOTE: only CORE migrations run here. Other extensions (e.g. reports) can
	// be appended to `sets` once cmd/migrate depends on them.

	runner := migration.NewRunner(pool, sets)
	if err := runner.Run(ctx); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("All migrations applied successfully.")
}
