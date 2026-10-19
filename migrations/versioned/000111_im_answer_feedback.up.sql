ALTER TABLE answer_feedback ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'web';
ALTER TABLE answer_feedback ADD COLUMN IF NOT EXISTS channel_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE answer_feedback ADD COLUMN IF NOT EXISTS last_event_id VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE answer_feedback ADD COLUMN IF NOT EXISTS last_event_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE answer_feedback DROP CONSTRAINT IF EXISTS answer_feedback_rating_check;
ALTER TABLE answer_feedback DROP CONSTRAINT IF EXISTS answer_feedback_status_check;
ALTER TABLE answer_feedback ADD CONSTRAINT answer_feedback_rating_check CHECK (rating IN ('useful', 'not_useful', ''));
ALTER TABLE answer_feedback ADD CONSTRAINT answer_feedback_status_check CHECK (status IN ('pending', 'resolved', 'ignored', 'withdrawn'));
CREATE TABLE IF NOT EXISTS im_feedback_targets (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    channel_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    user_id VARCHAR(128) NOT NULL,
    chat_id VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_im_feedback_targets_channel ON im_feedback_targets(channel_id);
