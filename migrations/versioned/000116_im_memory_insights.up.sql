ALTER TABLE messages ADD COLUMN memory_owner_id VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN memory_user_text TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN memory_eligible BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN generation_status VARCHAR(24) NOT NULL DEFAULT '';
ALTER TABLE memory_subjects ADD COLUMN forget_before TIMESTAMPTZ;
CREATE TABLE qa_questions (
 id VARCHAR(36) PRIMARY KEY, tenant_id BIGINT NOT NULL, message_id VARCHAR(36) NOT NULL UNIQUE,
 session_id VARCHAR(36) NOT NULL, request_id TEXT NOT NULL DEFAULT '', subject_id TEXT NOT NULL,
 user_id TEXT NOT NULL, user_name TEXT NOT NULL DEFAULT '', source TEXT NOT NULL, channel_id TEXT NOT NULL DEFAULT '',
 chat_id TEXT NOT NULL DEFAULT '', chat_type TEXT NOT NULL DEFAULT '', source_key TEXT NOT NULL UNIQUE,
 question TEXT NOT NULL, topic_id VARCHAR(36) NOT NULL DEFAULT '', classification TEXT NOT NULL DEFAULT 'pending',
 manual_topic BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_qa_question_scope ON qa_questions(tenant_id, created_at DESC);
CREATE INDEX idx_qa_question_topic ON qa_questions(tenant_id, topic_id, created_at DESC);
CREATE INDEX idx_qa_question_request ON qa_questions(tenant_id, session_id, request_id);
CREATE INDEX idx_qa_question_pending ON qa_questions(classification, created_at);
CREATE TABLE qa_topics (
 id VARCHAR(36) PRIMARY KEY, tenant_id BIGINT NOT NULL, normalized_key TEXT NOT NULL,
 title TEXT NOT NULL, guard TEXT NOT NULL DEFAULT '', embedding_model_id TEXT NOT NULL DEFAULT '',
 embedding JSONB, manual BOOLEAN NOT NULL DEFAULT FALSE, merged_into TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 CONSTRAINT idx_qa_topic_key UNIQUE(tenant_id, normalized_key)
);
CREATE TABLE qa_answers (
 id VARCHAR(36) PRIMARY KEY, tenant_id BIGINT NOT NULL, question_id VARCHAR(36) NOT NULL,
 session_id VARCHAR(36) NOT NULL, answer TEXT NOT NULL DEFAULT '', generation_status TEXT NOT NULL DEFAULT 'generating',
 delivery_status TEXT NOT NULL DEFAULT 'unknown', agent_id TEXT NOT NULL DEFAULT '', model_id TEXT NOT NULL DEFAULT '',
 model_name TEXT NOT NULL DEFAULT '', "references" JSONB, memory_count INTEGER NOT NULL DEFAULT 0,
 duration_ms BIGINT NOT NULL DEFAULT 0, usage JSONB, feedback_id TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_qa_answer_question ON qa_answers(tenant_id, question_id);
