ALTER TABLE plt_users
    ADD COLUMN IF NOT EXISTS totp_secret VARCHAR(64),
    ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS totp_backup_codes TEXT,
    ADD COLUMN IF NOT EXISTS failed_totp_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_totp_attempts >= 0);

CREATE TABLE IF NOT EXISTS plt_totp_challenges (
    token_hash VARCHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES plt_users(id) ON DELETE CASCADE,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_plt_totp_challenges_expires_at
    ON plt_totp_challenges(expires_at);
