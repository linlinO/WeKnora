UPDATE answer_feedback SET status = 'ignored' WHERE status = 'reviewed';
UPDATE answer_feedback SET status = 'pending' WHERE status = 'correcting';
DROP INDEX IF EXISTS idx_answer_feedback_workbench;
