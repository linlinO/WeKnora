-- Restore the pre-000109 built-in behavior on rollback.
UPDATE custom_agents
SET config = (COALESCE(config, '{}'::jsonb) - 'fallback_response') || jsonb_build_object(
    'web_search_enabled', true,
    'web_search_max_results', 5,
    'fallback_strategy', 'model'
)
WHERE id = 'builtin-quick-answer' AND is_builtin = true;
