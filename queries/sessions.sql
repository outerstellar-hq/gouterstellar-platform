-- name: CreateSession :one
INSERT INTO plt_sessions (token_hash, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING id, token_hash, user_id, created_at, expires_at;

-- name: FindSessionByTokenHash :one
SELECT id, token_hash, user_id, created_at, expires_at
FROM plt_sessions
WHERE token_hash = $1 AND expires_at > CURRENT_TIMESTAMP;

-- name: FindSessionByTokenHashIncludingExpired :one
SELECT id, token_hash, user_id, created_at, expires_at
FROM plt_sessions
WHERE token_hash = $1;

-- name: UpdateSessionExpiresAt :one
UPDATE plt_sessions SET expires_at = $2 WHERE token_hash = $1
RETURNING id, token_hash, user_id, created_at, expires_at;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM plt_sessions WHERE token_hash = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM plt_sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM plt_sessions WHERE expires_at <= CURRENT_TIMESTAMP;

-- name: ListSessionsForUser :many
SELECT token_hash, user_id, created_at, expires_at
FROM plt_sessions
WHERE user_id = $1 AND expires_at > CURRENT_TIMESTAMP
ORDER BY created_at DESC;
