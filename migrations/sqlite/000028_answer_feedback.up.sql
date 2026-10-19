CREATE TABLE IF NOT EXISTS answer_feedback (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    agent_id TEXT NOT NULL DEFAULT '',
    knowledge_base_id TEXT NOT NULL DEFAULT '',
    rating TEXT NOT NULL,
    question TEXT NOT NULL DEFAULT '',
    answer TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    reviewer_id TEXT NOT NULL DEFAULT '',
    review_note TEXT NOT NULL DEFAULT '',
    correction_knowledge_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, user_id, message_id)
);
CREATE INDEX IF NOT EXISTS idx_answer_feedback_tenant_status_created
    ON answer_feedback (tenant_id, status, created_at DESC);
