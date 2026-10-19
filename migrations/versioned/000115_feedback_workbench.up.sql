ALTER TABLE answer_feedback DROP CONSTRAINT IF EXISTS answer_feedback_status_check;
ALTER TABLE answer_feedback ADD CONSTRAINT answer_feedback_status_check
    CHECK (status IN ('pending', 'correcting', 'reviewed', 'resolved', 'ignored', 'withdrawn'));
CREATE INDEX IF NOT EXISTS idx_answer_feedback_workbench
    ON answer_feedback(tenant_id, rating, status, created_at DESC);
