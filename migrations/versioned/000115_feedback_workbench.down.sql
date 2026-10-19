UPDATE answer_feedback SET status = 'ignored' WHERE status = 'reviewed';
UPDATE answer_feedback SET status = 'pending' WHERE status = 'correcting';
ALTER TABLE answer_feedback DROP CONSTRAINT IF EXISTS answer_feedback_status_check;
ALTER TABLE answer_feedback ADD CONSTRAINT answer_feedback_status_check
    CHECK (status IN ('pending', 'resolved', 'ignored', 'withdrawn'));
DROP INDEX IF EXISTS idx_answer_feedback_workbench;
