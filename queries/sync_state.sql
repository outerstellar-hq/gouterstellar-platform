-- name: GetSyncState :one
SELECT state_key, state_value FROM plt_sync_state WHERE state_key = $1;

-- name: SetSyncState :exec
UPDATE plt_sync_state SET state_value = $2 WHERE state_key = $1;

-- name: UpsertSyncState :one
INSERT INTO plt_sync_state (state_key, state_value)
VALUES ($1, $2)
ON CONFLICT (state_key) DO UPDATE SET state_value = EXCLUDED.state_value
RETURNING state_key, state_value;
