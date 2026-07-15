-- name: FindUserByID :one
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout
FROM plt_users
WHERE id = $1;

-- name: FindUserByUsername :one
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout
FROM plt_users
WHERE username = $1;

-- name: FindUserByEmail :one
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout
FROM plt_users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO plt_users (id, username, email, password_hash, role, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;

-- name: FindAllUsers :many
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout
FROM plt_users
ORDER BY username ASC;

-- name: FindUserPage :many
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout
FROM plt_users
ORDER BY username ASC
LIMIT $1 OFFSET $2;

-- name: CountAllUsers :one
SELECT COUNT(*) FROM plt_users;

-- name: CountUsersByRole :one
SELECT COUNT(*) FROM plt_users WHERE role = $1;

-- name: UpdateUserRole :one
UPDATE plt_users SET role = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;

-- name: UpdateUserEnabled :one
UPDATE plt_users SET enabled = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;

-- name: UpdatePasswordHash :exec
UPDATE plt_users SET password_hash = $2 WHERE id = $1;

-- name: UpdateLastActivity :exec
UPDATE plt_users SET last_activity_at = CURRENT_TIMESTAMP WHERE id = $1;

-- name: DeleteUserByID :exec
DELETE FROM plt_users WHERE id = $1;

-- name: UpdateUsername :one
UPDATE plt_users SET username = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;

-- name: UpdateAvatarURL :one
UPDATE plt_users SET avatar_url = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;

-- name: UpdateNotificationPreferences :one
UPDATE plt_users
SET email_notifications_enabled = $2, push_notifications_enabled = $3
WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;

-- name: UpdatePreferences :one
UPDATE plt_users
SET language = $2, theme = $3, layout = $4
WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;

-- name: SeedAdminUser :one
INSERT INTO plt_users (id, username, email, password_hash, role, enabled)
VALUES ($1, $2, $3, $4, 'ADMIN', true)
ON CONFLICT (username) DO NOTHING
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout;
