CREATE TABLE IF NOT EXISTS answer_feedback (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    message_id VARCHAR(36) NOT NULL,
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    knowledge_base_id VARCHAR(36) NOT NULL DEFAULT '',
    rating VARCHAR(20) NOT NULL,
    question TEXT NOT NULL DEFAULT '',
    answer TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewer_id VARCHAR(128) NOT NULL DEFAULT '',
    review_note TEXT NOT NULL DEFAULT '',
    correction_knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT answer_feedback_rating_check CHECK (rating IN ('useful', 'not_useful')),
    CONSTRAINT answer_feedback_status_check CHECK (status IN ('pending', 'resolved', 'ignored')),
    CONSTRAINT answer_feedback_user_message_unique UNIQUE (tenant_id, user_id, message_id)
);
CREATE INDEX IF NOT EXISTS idx_answer_feedback_tenant_status_created
    ON answer_feedback (tenant_id, status, created_at DESC);
