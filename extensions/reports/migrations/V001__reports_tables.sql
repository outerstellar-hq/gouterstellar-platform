CREATE TABLE IF NOT EXISTS reports_snapshots (
    id            BIGSERIAL PRIMARY KEY,
    captured_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_count BIGINT NOT NULL,
    contact_count BIGINT NOT NULL
);
