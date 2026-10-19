ALTER TABLE answer_feedback ADD COLUMN source TEXT NOT NULL DEFAULT 'web';
ALTER TABLE answer_feedback ADD COLUMN channel_id TEXT NOT NULL DEFAULT '';
ALTER TABLE answer_feedback ADD COLUMN last_event_id TEXT NOT NULL DEFAULT '';
ALTER TABLE answer_feedback ADD COLUMN last_event_at INTEGER NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS im_feedback_targets (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    channel_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL,
    chat_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_im_feedback_targets_channel ON im_feedback_targets(channel_id);
