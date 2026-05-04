-- name: SaveNotification :one
INSERT INTO plt_notifications (id, user_id, title, body, type)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, title, body, type, read_at, created_at;

-- name: FindNotificationsByUserID :many
SELECT id, user_id, title, body, type, read_at, created_at
FROM plt_notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM plt_notifications WHERE user_id = $1 AND read_at IS NULL;

-- name: MarkNotificationRead :one
UPDATE plt_notifications SET read_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, title, body, type, read_at, created_at;

-- name: MarkAllNotificationsRead :execrows
UPDATE plt_notifications SET read_at = CURRENT_TIMESTAMP WHERE user_id = $1 AND read_at IS NULL;

-- name: DeleteNotification :execrows
DELETE FROM plt_notifications WHERE id = $1 AND user_id = $2;
