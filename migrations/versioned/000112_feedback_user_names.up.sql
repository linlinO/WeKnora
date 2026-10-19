CREATE TABLE IF NOT EXISTS im_feedback_user_names (
    tenant_id BIGINT NOT NULL,
    channel_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    name VARCHAR(100) NOT NULL,
    updated_by VARCHAR(36) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, channel_id, user_id)
);
