-- name: SavePasswordResetToken :one
INSERT INTO plt_password_reset_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token, expires_at, used, created_at;

-- name: FindPasswordResetByToken :one
SELECT id, user_id, token, expires_at, used, created_at
FROM plt_password_reset_tokens
WHERE token = $1;

-- name: MarkPasswordResetUsed :one
UPDATE plt_password_reset_tokens
SET used = true
WHERE token = $1
RETURNING id, user_id, token, expires_at, used, created_at;
