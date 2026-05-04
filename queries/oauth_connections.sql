-- name: FindOAuthByProviderSubject :one
SELECT id, user_id, provider, subject, email, created_at
FROM plt_oauth_connections
WHERE provider = $1 AND subject = $2;

-- name: SaveOAuthConnection :one
INSERT INTO plt_oauth_connections (user_id, provider, subject, email)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, provider, subject, email, created_at;

-- name: FindOAuthByUserID :many
SELECT id, user_id, provider, subject, email, created_at
FROM plt_oauth_connections
WHERE user_id = $1;

-- name: DeleteOAuthConnection :execrows
DELETE FROM plt_oauth_connections WHERE id = $1 AND user_id = $2;
