-- name: UpsertDeviceToken :one
INSERT INTO plt_device_tokens (user_id, platform, token, app_bundle)
VALUES ($1, $2, $3, $4)
ON CONFLICT (token) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    platform = EXCLUDED.platform,
    app_bundle = EXCLUDED.app_bundle,
    last_seen = CURRENT_TIMESTAMP
RETURNING id, user_id, platform, token, app_bundle, created_at, last_seen;

-- name: DeleteDeviceToken :execrows
DELETE FROM plt_device_tokens WHERE id = $1 AND user_id = $2;

-- name: FindDeviceTokensByUserID :many
SELECT id, user_id, platform, token, app_bundle, created_at, last_seen
FROM plt_device_tokens
WHERE user_id = $1;

-- name: DeleteAllDeviceTokensForUser :execrows
DELETE FROM plt_device_tokens WHERE user_id = $1;
