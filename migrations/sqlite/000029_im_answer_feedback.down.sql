DROP TABLE IF EXISTS im_feedback_targets;
ALTER TABLE answer_feedback DROP COLUMN source;
ALTER TABLE answer_feedback DROP COLUMN channel_id;
ALTER TABLE answer_feedback DROP COLUMN last_event_id;
ALTER TABLE answer_feedback DROP COLUMN last_event_at;
