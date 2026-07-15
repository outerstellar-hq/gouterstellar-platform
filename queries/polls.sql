-- name: CreatePoll :one
INSERT INTO plt_polls (sync_id, creator_id, question, multi_choice, deadline)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, sync_id, creator_id, question, multi_choice, closed_at, deadline, created_at, updated_at;

-- name: CreatePollOption :one
INSERT INTO plt_poll_options (poll_id, position, option_text)
VALUES ($1, $2, $3)
RETURNING id, poll_id, position, option_text;

-- name: FindPollBySyncID :one
SELECT id, sync_id, creator_id, question, multi_choice, closed_at, deadline, created_at, updated_at
FROM plt_polls
WHERE sync_id = $1;

-- name: LockPollBySyncID :one
SELECT id, sync_id, creator_id, question, multi_choice, closed_at, deadline, created_at, updated_at
FROM plt_polls
WHERE sync_id = $1
FOR UPDATE;

-- name: ListPollOptions :many
SELECT id, poll_id, position, option_text
FROM plt_poll_options
WHERE poll_id = $1
ORDER BY position;

-- name: FindPollOption :one
SELECT id, poll_id, position, option_text
FROM plt_poll_options
WHERE poll_id = $1 AND id = $2;

-- name: CastPollVote :execrows
INSERT INTO plt_poll_votes (poll_id, option_id, user_id)
VALUES ($1, $2, $3)
ON CONFLICT (poll_id, option_id, user_id) DO NOTHING;

-- name: RemovePollVote :execrows
DELETE FROM plt_poll_votes
WHERE poll_id = $1 AND option_id = $2 AND user_id = $3;

-- name: ListUserPollVotes :many
SELECT option_id
FROM plt_poll_votes
WHERE poll_id = $1 AND user_id = $2
ORDER BY option_id;

-- name: ListPollVoteCounts :many
SELECT option_id, COUNT(*)::INTEGER AS vote_count
FROM plt_poll_votes
WHERE poll_id = $1
GROUP BY option_id;

-- name: ClosePoll :execrows
UPDATE plt_polls
SET closed_at = COALESCE(closed_at, NOW()), updated_at = NOW()
WHERE id = $1;

-- name: DeletePoll :execrows
DELETE FROM plt_polls
WHERE id = $1;

-- name: ListOpenPolls :many
SELECT p.id, p.sync_id, p.creator_id, p.question, p.multi_choice, p.closed_at, p.deadline, p.created_at, p.updated_at,
       (SELECT COUNT(*)::INTEGER FROM plt_poll_votes v WHERE v.poll_id = p.id) AS total_votes
FROM plt_polls p
WHERE p.closed_at IS NULL AND (p.deadline IS NULL OR p.deadline > NOW())
ORDER BY p.created_at DESC
LIMIT $1 OFFSET $2;
