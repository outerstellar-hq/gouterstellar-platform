ALTER TABLE plt_users
    ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_login_attempts >= 0),
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
