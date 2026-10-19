CREATE TABLE IF NOT EXISTS im_feedback_user_names (
    tenant_id INTEGER NOT NULL,
    channel_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    updated_by TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, channel_id, user_id)
);
