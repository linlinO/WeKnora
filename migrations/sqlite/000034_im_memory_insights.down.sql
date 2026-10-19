DROP TABLE IF EXISTS qa_answers;
DROP TABLE IF EXISTS qa_questions;
DROP TABLE IF EXISTS qa_topics;
ALTER TABLE memory_subjects DROP COLUMN forget_before;
ALTER TABLE messages DROP COLUMN generation_status;
ALTER TABLE messages DROP COLUMN memory_eligible;
ALTER TABLE messages DROP COLUMN memory_user_text;
ALTER TABLE messages DROP COLUMN memory_owner_id;
