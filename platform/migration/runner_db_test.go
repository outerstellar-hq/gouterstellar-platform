package migration

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

// waitForDB polls the connection until the pool can acquire a connection and
// run a trivial query. The postgres Testcontainers module reports readiness
// before the server reliably accepts connections on some hosts (notably
// Docker Desktop on Windows), so we gate the test on an actual round-trip.
func waitForDB(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := pool.Ping(ctx); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("database did not become reachable within 30s")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func TestRunnerEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	waitForDB(t, ctx, pool)

	fakeFS := fstest.MapFS{
		"migrations/V001__create_table.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE test_items (id SERIAL PRIMARY KEY, name TEXT NOT NULL);`),
		},
		"migrations/V002__add_column.sql": &fstest.MapFile{
			Data: []byte(`ALTER TABLE test_items ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();`),
		},
	}

	sets := []extplatform.MigrationSet{{
		ExtensionID: "test-ext",
		FS:          fakeFS,
		Directory:   "migrations",
		Table:       "test_schema_migrations",
	}}

	runner := NewRunner(pool, sets)
	err = runner.Run(ctx)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both migrations should be recorded")

	// Second run: no-op
	err = runner.Run(ctx)
	require.NoError(t, err)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "re-run should not add new records")
}

func TestRunnerIsolatesExtensionHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	waitForDB(t, ctx, pool)

	coreFS := fstest.MapFS{
		"migrations/V001__core.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE core_items (id SERIAL PRIMARY KEY);`),
		},
	}
	reportsFS := fstest.MapFS{
		"migrations/V001__reports.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE reports_items (id SERIAL PRIMARY KEY);`),
		},
	}

	sets := []extplatform.MigrationSet{
		{ExtensionID: "reports", FS: reportsFS, Directory: "migrations", Table: "schema_migrations_reports"},
		{ExtensionID: "platform-core", FS: coreFS, Directory: "migrations", Table: "schema_migrations_core"},
	}

	runner := NewRunner(pool, sets)
	err = runner.Run(ctx)
	require.NoError(t, err)

	var coreCount, reportsCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations_core").Scan(&coreCount)
	require.NoError(t, err)
	assert.Equal(t, 1, coreCount)

	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations_reports").Scan(&reportsCount)
	require.NoError(t, err)
	assert.Equal(t, 1, reportsCount)
}
