-- name: CreateApiKey :one
INSERT INTO plt_api_keys (user_id, key_hash, key_prefix, name, enabled)
VALUES ($1, $2, $3, $4, true)
RETURNING id, user_id, key_hash, key_prefix, name, enabled, created_at, last_used_at;

-- name: FindApiKeyByHash :one
SELECT id, user_id, key_hash, key_prefix, name, enabled, created_at, last_used_at
FROM plt_api_keys
WHERE key_hash = $1 AND enabled = true;

-- name: FindApiKeysByUserID :many
SELECT id, user_id, key_hash, key_prefix, name, enabled, created_at, last_used_at
FROM plt_api_keys
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: DeleteApiKey :execrows
DELETE FROM plt_api_keys WHERE id = $1 AND user_id = $2;

-- name: UpdateApiKeyLastUsed :exec
UPDATE plt_api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1;
