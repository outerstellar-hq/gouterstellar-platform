-- name: SaveOutbox :exec
INSERT INTO plt_outbox (id, payload_type, payload, status)
VALUES ($1, $2, $3, $4);

-- name: ListPendingOutbox :many
SELECT id, payload_type, payload, status, created_at, processed_at, retry_count, last_error
FROM plt_outbox
WHERE status = 'PENDING'
ORDER BY created_at ASC
LIMIT $1;

-- name: MarkOutboxProcessed :one
UPDATE plt_outbox
SET status = 'PROCESSED', processed_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, payload_type, payload, status, created_at, processed_at, retry_count, last_error;

-- name: MarkOutboxFailed :one
UPDATE plt_outbox
SET status = 'FAILED', last_error = $2, retry_count = retry_count + 1
WHERE id = $1
RETURNING id, payload_type, payload, status, created_at, processed_at, retry_count, last_error;

-- name: GetOutboxStats :one
SELECT
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE status = 'PENDING') AS pending,
    COUNT(*) FILTER (WHERE status = 'PROCESSED') AS processed,
    COUNT(*) FILTER (WHERE status = 'FAILED') AS failed
FROM plt_outbox;

-- name: ListFailedOutbox :many
SELECT id, payload_type, payload, status, created_at, processed_at, retry_count, last_error
FROM plt_outbox
WHERE status = 'FAILED'
ORDER BY created_at ASC
LIMIT $1;

-- name: ListDeadLetterOutbox :many
SELECT id, payload_type, payload, status, created_at, processed_at, retry_count, last_error
FROM plt_outbox
WHERE status = 'DEAD_LETTER'
ORDER BY created_at ASC
LIMIT $1;

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
RETURNING id, payload_type, payload, status, created_at, processed_at, retry_count, last_error;

-- name: UpdateOutboxStatus :one
UPDATE plt_outbox
SET status = $2, last_error = $3
WHERE id = $1
RETURNING id, payload_type, payload, status, created_at, processed_at, retry_count, last_error;
