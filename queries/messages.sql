-- name: ListMessages :many
SELECT id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict
FROM plt_messages
WHERE deleted = false
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: SearchMessages :many
SELECT id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict
FROM plt_messages
WHERE deleted = false
AND (content ILIKE '%' || $1::text || '%' OR author ILIKE '%' || $1::text || '%')
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSearchMessages :one
SELECT COUNT(*) FROM plt_messages
WHERE deleted = false
AND (content ILIKE '%' || $1::text || '%' OR author ILIKE '%' || $1::text || '%');

-- name: CountMessages :one
SELECT COUNT(*) FROM plt_messages WHERE deleted = false;

-- name: FindBySyncID :one
SELECT id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict
FROM plt_messages
WHERE sync_id = $1;

-- name: CreateServerMessage :one
INSERT INTO plt_messages (sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version)
VALUES ($1, $2, $3, $4, false, false, 1)
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: CreateLocalMessage :one
INSERT INTO plt_messages (sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version)
VALUES ($1, $2, $3, $4, true, false, 1)
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: UpsertSyncedMessage :one
INSERT INTO plt_messages (sync_id, author, content, updated_at_epoch_ms, dirty, deleted, version)
VALUES ($1, $2, $3, $4, false, $5, 1)
ON CONFLICT (sync_id) DO UPDATE SET
    author = EXCLUDED.author,
    content = EXCLUDED.content,
    updated_at_epoch_ms = EXCLUDED.updated_at_epoch_ms,
    dirty = false,
    deleted = EXCLUDED.deleted,
    version = plt_messages.version + 1,
    sync_conflict = NULL
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: FindChangesSince :many
SELECT id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict
FROM plt_messages
WHERE updated_at_epoch_ms > $1
ORDER BY updated_at_epoch_ms ASC;

-- name: ListDirtyMessages :many
SELECT id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict
FROM plt_messages
WHERE dirty = true AND deleted = false
ORDER BY updated_at_epoch_ms ASC;

-- name: CountDirtyMessages :one
SELECT COUNT(*) FROM plt_messages WHERE dirty = true AND deleted = false;

-- name: SoftDeleteMessage :one
UPDATE plt_messages
SET deleted = true, deleted_at = CURRENT_TIMESTAMP, dirty = true, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: RestoreMessage :one
UPDATE plt_messages
SET deleted = false, deleted_at = NULL, dirty = true, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: UpdateMessage :one
UPDATE plt_messages
SET author = $2, content = $3, updated_at_epoch_ms = $4, dirty = $5, version = version + 1
WHERE sync_id = $1 AND version = $6
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: MarkConflictMessage :one
UPDATE plt_messages
SET sync_conflict = $2, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: ResolveConflictMessage :one
UPDATE plt_messages
SET sync_conflict = NULL, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, author, content, created_at, updated_at_epoch_ms, deleted, dirty, deleted_at, version, sync_conflict;

-- name: MarkCleanMessages :exec
UPDATE plt_messages SET dirty = false WHERE dirty = true;
