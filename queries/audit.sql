-- name: LogAudit :one
INSERT INTO plt_audit_log (actor_id, actor_username, target_id, target_username, action, detail)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, actor_id, actor_username, target_id, target_username, action, detail, created_at;

-- name: FindRecentAudit :many
SELECT id, actor_id, actor_username, target_id, target_username, action, detail, created_at
FROM plt_audit_log
ORDER BY created_at DESC
LIMIT $1;

-- name: FindAuditPage :many
SELECT id, actor_id, actor_username, target_id, target_username, action, detail, created_at
FROM plt_audit_log
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAllAudit :one
SELECT COUNT(*) FROM plt_audit_log;
