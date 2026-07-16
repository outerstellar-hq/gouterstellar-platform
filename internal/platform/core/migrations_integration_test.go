package core_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/platform/core"
	"github.com/outerstellar-hq/gouterstellar-platform/platform/migration"
)

func TestPrefixedSyncIDMigrationUpgradesExistingSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database-backed migration test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connectionString, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.Eventually(t, func() bool { return pool.Ping(ctx) == nil }, 30*time.Second, 500*time.Millisecond)

	_, err = pool.Exec(ctx, `
		CREATE TABLE plt_messages (sync_id VARCHAR(36) NOT NULL UNIQUE);
		CREATE TABLE plt_contacts (sync_id VARCHAR(36) NOT NULL UNIQUE);
		CREATE TABLE plt_message_votes (
			message_sync_id VARCHAR(36) NOT NULL,
			CONSTRAINT plt_message_votes_message_sync_id_fkey
				FOREIGN KEY (message_sync_id) REFERENCES plt_messages(sync_id) ON DELETE CASCADE
		);
		CREATE TABLE schema_migrations_core (
			version BIGINT PRIMARY KEY,
			filename TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO schema_migrations_core (version, filename)
		SELECT version, 'legacy' FROM generate_series(1, 8) AS version;
	`)
	require.NoError(t, err)

	coreExtension := core.NewExtension()
	require.NoError(t, migration.NewRunner(pool, coreExtension.Manifest().Migrations).Run(ctx))

	for _, column := range []struct {
		table string
		name  string
	}{
		{table: "plt_messages", name: "sync_id"},
		{table: "plt_contacts", name: "sync_id"},
		{table: "plt_message_votes", name: "message_sync_id"},
	} {
		var length int
		err = pool.QueryRow(ctx, `
			SELECT character_maximum_length
			FROM information_schema.columns
			WHERE table_name = $1 AND column_name = $2
		`, column.table, column.name).Scan(&length)
		require.NoError(t, err)
		assert.Equal(t, 64, length, column.table+"."+column.name)
	}

	prefixedID := "srv_" + strings.Repeat("a", 36)
	for _, statement := range []string{
		"INSERT INTO plt_messages (sync_id) VALUES ($1)",
		"INSERT INTO plt_contacts (sync_id) VALUES ($1)",
		"INSERT INTO plt_message_votes (message_sync_id) VALUES ($1)",
	} {
		_, err = pool.Exec(ctx, statement, prefixedID)
		require.NoError(t, err)
	}
}
