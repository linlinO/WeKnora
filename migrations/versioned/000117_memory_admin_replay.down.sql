DROP INDEX IF EXISTS idx_replay_running;
ALTER TABLE knowledge_regression_results DROP COLUMN evaluation, DROP COLUMN duration_ms, DROP COLUMN "references", DROP COLUMN expected_answer, DROP COLUMN question;
ALTER TABLE knowledge_regression_runs DROP COLUMN input, DROP COLUMN config_hash, DROP COLUMN model_id, DROP COLUMN agent_id, DROP COLUMN candidate_hash, DROP COLUMN kind;
DROP TABLE memory_admin_grants;
ALTER TABLE qa_answers DROP COLUMN memory_diagnostic;
ALTER TABLE messages DROP COLUMN memory_diagnostic;
