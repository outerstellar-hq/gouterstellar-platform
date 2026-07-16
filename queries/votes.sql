-- name: LockMessageForVote :one
SELECT id
FROM plt_messages
WHERE sync_id = $1 AND deleted = false
FOR UPDATE;

-- name: FindMessageVote :one
SELECT id, message_sync_id, user_id, direction, created_at
FROM plt_message_votes
WHERE user_id = $1 AND message_sync_id = $2;

-- name: CreateMessageVote :one
INSERT INTO plt_message_votes (message_sync_id, user_id, direction)
VALUES ($1, $2, $3)
RETURNING id, message_sync_id, user_id, direction, created_at;

-- name: UpdateMessageVote :execrows
UPDATE plt_message_votes
SET direction = $3
WHERE user_id = $1 AND message_sync_id = $2;

-- name: DeleteMessageVote :execrows
DELETE FROM plt_message_votes
WHERE user_id = $1 AND message_sync_id = $2;

-- name: ListMessageVoteCounts :many
SELECT
    message_sync_id,
    COUNT(*) FILTER (WHERE direction = 1)::INTEGER AS upvotes,
    COUNT(*) FILTER (WHERE direction = -1)::INTEGER AS downvotes
FROM plt_message_votes
WHERE message_sync_id = ANY(sqlc.arg(message_sync_ids)::text[])
GROUP BY message_sync_id;

-- name: ListUserMessageVotes :many
SELECT message_sync_id, direction
FROM plt_message_votes
WHERE user_id = sqlc.arg(user_id)
  AND message_sync_id = ANY(sqlc.arg(message_sync_ids)::text[]);
