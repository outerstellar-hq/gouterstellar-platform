-- name: CreateTOTPChallenge :exec
INSERT INTO plt_totp_challenges (token_hash, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: TakeTOTPChallengeAttempt :one
UPDATE plt_totp_challenges
SET attempt_count = attempt_count + 1
WHERE token_hash = $1
  AND expires_at > CURRENT_TIMESTAMP
  AND attempt_count < $2
RETURNING token_hash, user_id, attempt_count, expires_at, created_at;

-- name: DeleteTOTPChallenge :execrows
DELETE FROM plt_totp_challenges WHERE token_hash = $1;

-- name: DeleteExpiredTOTPChallenges :execrows
DELETE FROM plt_totp_challenges WHERE expires_at <= CURRENT_TIMESTAMP;

-- name: EnableUserTOTP :exec
UPDATE plt_users
SET totp_secret = $2,
    totp_enabled = TRUE,
    totp_backup_codes = $3,
    failed_totp_attempts = 0
WHERE id = $1;

-- name: DisableUserTOTP :exec
UPDATE plt_users
SET totp_secret = NULL,
    totp_enabled = FALSE,
    totp_backup_codes = NULL,
    failed_totp_attempts = 0
WHERE id = $1;

-- name: IncrementFailedTOTPAttempts :one
UPDATE plt_users
SET failed_totp_attempts = failed_totp_attempts + 1
WHERE id = $1
RETURNING failed_totp_attempts;

-- name: ResetFailedTOTPAttempts :exec
UPDATE plt_users SET failed_totp_attempts = 0 WHERE id = $1;

-- name: ReplaceTOTPBackupCodes :execrows
UPDATE plt_users
SET totp_backup_codes = sqlc.narg(new_codes)
WHERE id = sqlc.arg(user_id)
  AND totp_backup_codes = sqlc.arg(expected_codes);
