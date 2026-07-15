# Outbox Correctness Design (Workstream C)

**Date:** 2026-07-11
**Status:** Approved
**Workstream:** C of 5

## Problem

The transactional outbox is architecturally broken in 3 ways:
1. Domain change + outbox event are NOT in the same transaction
2. No locking (FOR UPDATE SKIP LOCKED) — concurrent workers duplicate-process
3. First failure marks FAILED (never retried), no dead-letter path

## Fixes

### Fix 1: Transactional outbox writes

Wrap domain mutation + outbox insert in `TransactionManager.InTransaction`. The outbox repo's `SaveOutbox` needs to accept a `pgx.Tx` or work within the transaction's context. The cleanest approach: add `SaveOutboxTx(ctx, tx, ...)` methods that accept a transaction, or make the repo methods aware of transactions via the context.

**Approach:** Add a `WithTx(tx pgx.Tx)` method to the outbox repo that returns a new repo instance bound to the transaction. Then wrap each domain-write + outbox-save pair in `InTransaction`.

For message service: wrap `repo.CreateServerMessage` + `saveOutboxEntry` in `txMgr.InTransaction`.
For contact service: wrap `repo.CreateContact` + `saveContactOutboxEntry` in `txMgr.InTransaction`.

### Fix 2: Safe claiming with FOR UPDATE SKIP LOCKED

Change the `ListPendingOutbox` query to use `FOR UPDATE SKIP LOCKED` so concurrent workers don't pick up the same rows. This requires a new SQL query and `sqlc generate`.

New query:
```sql
-- name: ClaimPendingOutbox :many
UPDATE plt_outbox
SET status = 'PROCESSING', retry_count = retry_count + 1
WHERE id IN (
    SELECT id FROM plt_outbox
    WHERE status = 'PENDING' OR (status = 'PROCESSING' AND retry_count < 5)
    ORDER BY created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

This atomically claims rows by moving them to PROCESSING status. Other workers skip locked rows.

### Fix 3: Retry + dead-letter

Update the processor:
- Claim rows (PROCESSING status)
- Process each
- On success: mark PROCESSED
- On failure with retry_count < max (5): mark PENDING (retry)
- On failure with retry_count >= max: mark DEAD_LETTER (permanent failure)

Add a `DEAD_LETTER` status. Update the MarkOutboxFailed query to accept the status to set.

## Out of scope

- Actually implementing message/contact sync processing logic (the processEntry switch is still a stub)
- Adding a dead-letter monitoring UI
- Adding outbox integration tests (Testcontainers) — deferred to Workstream D
