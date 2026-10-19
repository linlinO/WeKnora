UPDATE custom_agents
SET config = COALESCE(config, '{}'::jsonb) || '{"knowledge_only":true}'::jsonb
WHERE id = 'builtin-quick-answer' AND is_builtin = true;
