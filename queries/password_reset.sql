-- name: SavePasswordResetToken :one
INSERT INTO plt_password_reset_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token, expires_at, used, created_at;

-- name: InvalidatePasswordResetTokensForUser :exec
UPDATE plt_password_reset_tokens
SET used = true
WHERE user_id = $1 AND used = false;

-- name: ClaimPasswordResetToken :one
UPDATE plt_password_reset_tokens
SET used = true
WHERE token = $1
  AND used = false
  AND expires_at > CURRENT_TIMESTAMP
RETURNING user_id;
