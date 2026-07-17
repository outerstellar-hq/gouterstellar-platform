-- name: FindUserByID :one
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts
FROM plt_users
WHERE id = $1;

-- name: FindUserByUsername :one
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts
FROM plt_users
WHERE username = $1;

-- name: FindUserByEmail :one
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts
FROM plt_users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO plt_users (id, username, email, password_hash, role, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: FindAllUsers :many
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts
FROM plt_users
ORDER BY username ASC;

-- name: FindUserPage :many
SELECT id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts
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
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: UpdateUserEnabled :one
UPDATE plt_users SET enabled = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: UpdatePasswordHash :exec
UPDATE plt_users
SET password_hash = $2, failed_login_attempts = 0, locked_until = NULL
WHERE id = $1;

-- name: UpdateLastActivity :exec
UPDATE plt_users SET last_activity_at = CURRENT_TIMESTAMP WHERE id = $1;

-- name: DeleteUserByID :exec
DELETE FROM plt_users WHERE id = $1;

-- name: UpdateUsername :one
UPDATE plt_users SET username = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: UpdateEmail :one
UPDATE plt_users SET email = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: UpdateAvatarURL :one
UPDATE plt_users SET avatar_url = $2 WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: UpdateNotificationPreferences :one
UPDATE plt_users
SET email_notifications_enabled = $2, push_notifications_enabled = $3
WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: UpdatePreferences :one
UPDATE plt_users
SET language = $2, theme = $3, layout = $4
WHERE id = $1
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: SeedAdminUser :one
INSERT INTO plt_users (id, username, email, password_hash, role, enabled)
VALUES ($1, $2, $3, $4, 'ADMIN', true)
ON CONFLICT (username) DO NOTHING
RETURNING id, username, email, password_hash, role, enabled, created_at, last_activity_at,
       avatar_url, email_notifications_enabled, push_notifications_enabled,
       language, theme, layout, failed_login_attempts, locked_until,
       totp_secret, totp_enabled, totp_backup_codes, failed_totp_attempts;

-- name: IncrementFailedLoginAttempts :one
UPDATE plt_users
SET failed_login_attempts = failed_login_attempts + 1
WHERE id = $1
RETURNING failed_login_attempts;

-- name: ResetLoginFailures :exec
UPDATE plt_users SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1;

-- name: LockUserUntil :exec
UPDATE plt_users SET locked_until = $2 WHERE id = $1;
