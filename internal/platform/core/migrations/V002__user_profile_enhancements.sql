-- =============================================================================
-- User profile enhancements
-- Adds avatar URL and notification preference columns to the plt_users table.
-- =============================================================================
ALTER TABLE plt_users ADD COLUMN IF NOT EXISTS avatar_url                  VARCHAR(512);
ALTER TABLE plt_users ADD COLUMN IF NOT EXISTS email_notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE plt_users ADD COLUMN IF NOT EXISTS push_notifications_enabled  BOOLEAN NOT NULL DEFAULT TRUE;
