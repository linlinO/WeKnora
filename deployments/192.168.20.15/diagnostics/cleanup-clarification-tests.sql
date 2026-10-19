\set ON_ERROR_STOP on
BEGIN;
CREATE TEMP TABLE clarification_test_sessions(id text PRIMARY KEY);
INSERT INTO clarification_test_sessions VALUES
('4ae9a416-96e2-416a-bc9e-9e5452aaf916'),
('929a066c-8547-463a-a0b9-1c5cbc6e3ff0'),
('fda23ae1-4400-4d74-913c-5ff18bfa51c6'),
('661404d7-0238-46ec-a5ff-1406dab7e637'),
('89de99db-56d8-4b08-80c7-6e63ed6cb3ab'),
('4fed1a00-9ba6-4926-84ca-0a7ec8973c46'),
('d91f60ce-81cb-492f-bdeb-73da7297cc7c'),
('bfc7ff40-60cd-496e-8ac3-c0ebe7e78c71'),
('b653c564-78da-4f38-96f1-94fcd0e83542');
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM qa_answers a JOIN clarification_test_sessions s ON a.session_id=s.id WHERE a.tenant_id=10000 AND coalesce(a.feedback_id,'')<>'') THEN
  RAISE EXCEPTION 'A test answer now has feedback; preserve it for review';
 END IF;
 IF (SELECT count(*) FROM qa_questions q JOIN clarification_test_sessions s ON q.session_id=s.id WHERE q.tenant_id=10000)>22 THEN
  RAISE EXCEPTION 'Unexpected number of test questions; preserve for review';
 END IF;
END $$;
-- Emit recoverable test data into the administrator-only cleanup log before
-- deletion. All IDs above were created by this task's temporary API key.
SELECT json_build_object('table','qa_questions','row',row_to_json(q)) FROM qa_questions q JOIN clarification_test_sessions s ON q.session_id=s.id WHERE q.tenant_id=10000;
SELECT json_build_object('table','qa_answers','row',row_to_json(a)) FROM qa_answers a JOIN clarification_test_sessions s ON a.session_id=s.id WHERE a.tenant_id=10000;
DELETE FROM qa_answers a USING clarification_test_sessions s WHERE a.tenant_id=10000 AND a.session_id=s.id;
DELETE FROM qa_questions q USING clarification_test_sessions s WHERE q.tenant_id=10000 AND q.session_id=s.id;
DELETE FROM tenant_api_keys WHERE tenant_id=10000 AND name='codex-clarification-smoke-20261014';
COMMIT;
