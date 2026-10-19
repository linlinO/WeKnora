UPDATE custom_agents SET config = json_set(COALESCE(config, '{}'), '$.knowledge_only', json('true'))
WHERE id = 'builtin-quick-answer' AND is_builtin = 1;
