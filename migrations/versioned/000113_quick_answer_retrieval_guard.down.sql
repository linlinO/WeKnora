UPDATE custom_agents SET config = config - 'knowledge_only'
WHERE id = 'builtin-quick-answer' AND is_builtin = true;
