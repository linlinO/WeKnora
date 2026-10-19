UPDATE custom_agents SET config = json_remove(config, '$.knowledge_only')
WHERE id = 'builtin-quick-answer' AND is_builtin = 1;
