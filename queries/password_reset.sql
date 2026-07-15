-- name: SavePasswordResetToken :one
INSERT INTO plt_password_reset_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token, expires_at, used, created_at;

-- name: ConsumePasswordReset :one
WITH consumed AS (
    UPDATE plt_password_reset_tokens
    SET used = true
    WHERE token = $1
      AND used = false
      AND expires_at > CURRENT_TIMESTAMP
    RETURNING user_id
)
UPDATE plt_users
SET password_hash = $2
FROM consumed
WHERE plt_users.id = consumed.user_id
RETURNING plt_users.id, username, email, password_hash, role, enabled, created_at,
          last_activity_at, avatar_url, email_notifications_enabled,
          push_notifications_enabled, language, theme, layout;
