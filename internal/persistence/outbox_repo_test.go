package persistence

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestOutboxRepositoryIntegration exercises the OutboxRepository against a
// real Postgres instance: save, claim, mark processed, retry, and dead-letter.
//
// The dead-letter policy lives in the service layer
// (internal/service/outbox_processor.go failureStatus), but the repository
// primitives it is built on are tested here: ClaimPending atomically
// increments retry_count and stops re-claiming an entry once retry_count
// reaches 5, and UpdateStatus("DEAD_LETTER", ...) is what ListDeadLetter
// surfaces.
func TestOutboxRepositoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()
	waitForDB(t, ctx, pool)

	// Create the outbox table, matching the core migration's shape exactly.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS plt_outbox (
			id            UUID         PRIMARY KEY,
			payload_type  VARCHAR(255) NOT NULL,
			payload       TEXT         NOT NULL,
			status        VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
			created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			processed_at  TIMESTAMP,
			retry_count   INT               DEFAULT 0,
			last_error    TEXT,
			deleted_at    TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_plt_outbox_unprocessed ON plt_outbox(created_at, processed_at);
		CREATE INDEX IF NOT EXISTS idx_plt_outbox_status       ON plt_outbox(status);
	`)
	require.NoError(t, err)

	repo := NewOutboxRepository(pool)

	// cleanOutbox wipes the outbox table so each subtest starts from a known
	// empty state. Subtests would otherwise leak PROCESSING rows into each
	// other because ClaimPending re-claims PROCESSING rows (retry_count < 5).
	cleanOutbox := func(t *testing.T) {
		t.Helper()
		_, err := pool.Exec(ctx, "DELETE FROM plt_outbox")
		require.NoError(t, err)
	}

	t.Run("save_claim_process", func(t *testing.T) {
		cleanOutbox(t)
		id := uuid.New()
		require.NoError(t, repo.SaveOutbox(ctx, id, "MESSAGE_SYNC", `{"syncId":"m1"}`, "PENDING"))

		// ClaimPending returns the entry and flips it to PROCESSING.
		claimed, err := repo.ClaimPending(ctx, 10)
		require.NoError(t, err)
		require.Len(t, claimed, 1, "exactly the one entry we saved should be claimed")
		assert.Equal(t, id, claimed[0].ID)
		assert.Equal(t, "PROCESSING", claimed[0].Status, "ClaimPending must set status to PROCESSING")
		assert.NotNil(t, claimed[0].RetryCount, "retry_count should be non-null after claim")
		assert.Equal(t, int32(1), *claimed[0].RetryCount, "first claim should set retry_count to 1")

		// Mark processed; processed_at should be set.
		processed, err := repo.MarkProcessed(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "PROCESSED", processed.Status)
		assert.True(t, processed.ProcessedAt.Valid, "processed_at should be populated")

		// After processing, ClaimPending no longer returns it (status is PROCESSED,
		// which is neither PENDING nor PROCESSING).
		again, err := repo.ClaimPending(ctx, 10)
		require.NoError(t, err)
		assert.Empty(t, again, "processed entry should not be re-claimed")
	})

	t.Run("retry_resets_to_pending_and_increments", func(t *testing.T) {
		cleanOutbox(t)
		id := uuid.New()
		require.NoError(t, repo.SaveOutbox(ctx, id, "CONTACT_SYNC", `{"syncId":"c1"}`, "PENDING"))

		// First claim -> retry_count=1, status PROCESSING.
		claimed, err := repo.ClaimPending(ctx, 10)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		assert.Equal(t, int32(1), *claimed[0].RetryCount)

		// Simulate a failure: the service sets status back to PENDING so the
		// entry becomes eligible again. (Repo's MarkFailed would instead set
		// FAILED; the service uses UpdateStatus to choose PENDING vs
		// DEAD_LETTER based on retry_count.)
		errMsg := "transient downstream error"
		_, err = repo.UpdateStatus(ctx, id, "PENDING", &errMsg)
		require.NoError(t, err)

		// Second claim -> retry_count=2.
		claimed, err = repo.ClaimPending(ctx, 10)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		assert.Equal(t, "PROCESSING", claimed[0].Status)
		assert.Equal(t, int32(2), *claimed[0].RetryCount, "second claim should increment retry_count to 2")
	})

	t.Run("claim_stops_reclaiming_after_five_retries", func(t *testing.T) {
		// This is the repository primitive the service's dead-letter policy
		// relies on: ClaimPending only re-claims PROCESSING rows whose
		// retry_count < 5. Once an entry has been claimed 5 times and left in
		// PROCESSING, it is no longer returned by ClaimPending.
		cleanOutbox(t)
		id := uuid.New()
		require.NoError(t, repo.SaveOutbox(ctx, id, "MESSAGE_SYNC", `{"syncId":"dl"}`, "PENDING"))

		for i := 1; i <= 5; i++ {
			claimed, err := repo.ClaimPending(ctx, 10)
			require.NoError(t, err)
			require.Len(t, claimed, 1, "claim %d should return the entry", i)
			assert.Equal(t, int32(i), *claimed[0].RetryCount, "claim %d: retry_count mismatch", i)
			// Leave the row in PROCESSING between claims (do NOT reset to PENDING).
		}

		// Sixth claim: retry_count is now 5, so the row is no longer eligible
		// for re-claim (the query predicate is retry_count < 5).
		claimed, err := repo.ClaimPending(ctx, 10)
		require.NoError(t, err)
		assert.Empty(t, claimed, "entry with retry_count=5 should not be re-claimed (dead-letter threshold)")

		// The service would now mark it DEAD_LETTER via UpdateStatus.
		_, err = repo.UpdateStatus(ctx, id, "DEAD_LETTER", nil)
		require.NoError(t, err)

		// And it surfaces in ListDeadLetter.
		dead, err := repo.ListDeadLetter(ctx, 10)
		require.NoError(t, err)
		require.NotEmpty(t, dead, "DEAD_LETTER entry should appear in ListDeadLetter")
		assert.Equal(t, id, dead[0].ID)
		assert.Equal(t, "DEAD_LETTER", dead[0].Status)
	})

	t.Run("GetStats_counts_by_status", func(t *testing.T) {
		cleanOutbox(t)
		id := uuid.New()
		require.NoError(t, repo.SaveOutbox(ctx, id, "MESSAGE_SYNC", `{}`, "PENDING"))

		stats, err := repo.GetStats(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), stats.Total)
		assert.Equal(t, int64(1), stats.Pending)
		assert.Equal(t, int64(0), stats.Processed)

		// Process it, then stats should reflect exactly one processed row.
		_, err = repo.MarkProcessed(ctx, id)
		require.NoError(t, err)
		stats, err = repo.GetStats(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), stats.Total)
		assert.Equal(t, int64(0), stats.Pending)
		assert.Equal(t, int64(1), stats.Processed)
	})

	t.Run("ListPending_only_returns_pending", func(t *testing.T) {
		cleanOutbox(t)
		pendingID := uuid.New()
		require.NoError(t, repo.SaveOutbox(ctx, pendingID, "MESSAGE_SYNC", `{}`, "PENDING"))
		processedID := uuid.New()
		require.NoError(t, repo.SaveOutbox(ctx, processedID, "MESSAGE_SYNC", `{}`, "PENDING"))
		_, err := repo.MarkProcessed(ctx, processedID)
		require.NoError(t, err)

		pending, err := repo.ListPending(ctx, 10)
		require.NoError(t, err)
		require.Len(t, pending, 1, "only the still-PENDING row should be listed")
		assert.Equal(t, pendingID, pending[0].ID)
		for _, p := range pending {
			assert.Equal(t, "PENDING", p.Status, "ListPending must only return PENDING rows")
		}
	})

	t.Run("MarkFailed_sets_failed_status_and_increments", func(t *testing.T) {
		cleanOutbox(t)
		id := uuid.New()
		require.NoError(t, repo.SaveOutbox(ctx, id, "CONTACT_SYNC", `{}`, "PENDING"))

		errMsg := "boom"
		failed, err := repo.MarkFailed(ctx, id, &errMsg)
		require.NoError(t, err)
		assert.Equal(t, "FAILED", failed.Status)
		assert.NotNil(t, failed.LastError)
		assert.Equal(t, "boom", *failed.LastError)
		// MarkOutboxFailed increments retry_count independently of ClaimPending.
		assert.NotNil(t, failed.RetryCount)
		assert.Equal(t, int32(1), *failed.RetryCount)
	})
}
