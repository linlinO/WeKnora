CREATE INDEX IF NOT EXISTS idx_answer_feedback_workbench
    ON answer_feedback(tenant_id, rating, status, created_at DESC);
