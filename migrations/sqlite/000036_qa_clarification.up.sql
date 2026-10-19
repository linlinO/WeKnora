CREATE TABLE IF NOT EXISTS qa_clarification_states (
 scope_key text PRIMARY KEY,
 input text NOT NULL,
 missing text NOT NULL DEFAULT '',
 rounds integer NOT NULL DEFAULT 0,
 expires_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_qa_clarification_expiry ON qa_clarification_states(expires_at);
