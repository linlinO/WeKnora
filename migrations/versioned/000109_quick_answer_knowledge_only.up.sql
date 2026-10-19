-- Migration: 000109_quick_answer_knowledge_only
-- The fast-answer agent is the low-cost, deterministic RAG entry point. It
-- must not silently fall back to web search or a free-form model answer when
-- retrieval has no evidence.
UPDATE custom_agents
SET config = COALESCE(config, '{}'::jsonb) || jsonb_build_object(
    'web_search_enabled', false,
    'web_search_max_results', 0,
    'fallback_strategy', 'fixed',
    'fallback_response', '知识库中没有找到足够可靠的依据，我无法确认这个问题。请补充文档或联系管理员纠正知识库。'
)
WHERE id = 'builtin-quick-answer' AND is_builtin = true;
