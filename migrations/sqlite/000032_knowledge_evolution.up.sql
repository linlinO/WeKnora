CREATE TABLE IF NOT EXISTS knowledge_evolution_proposals (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id TEXT NOT NULL,
    feedback_id TEXT NOT NULL,
    question TEXT NOT NULL DEFAULT '',
    question_hash TEXT NOT NULL DEFAULT '',
    original_answer TEXT NOT NULL DEFAULT '',
    candidate_answer TEXT NOT NULL DEFAULT '',
    similar_questions TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'draft',
    conflict_status TEXT NOT NULL DEFAULT 'none',
    conflict_summary TEXT NOT NULL DEFAULT '',
    conflict_knowledge_ids TEXT NOT NULL DEFAULT '[]',
    version_group_id TEXT NOT NULL DEFAULT '',
    supersedes_knowledge_id TEXT NOT NULL DEFAULT '',
    correction_knowledge_id TEXT NOT NULL DEFAULT '',
    regression_run_id TEXT NOT NULL DEFAULT '',
    auto_generated INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT '',
    reviewer_id TEXT NOT NULL DEFAULT '',
    review_note TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_evolution_proposal_feedback ON knowledge_evolution_proposals (tenant_id, feedback_id);
CREATE INDEX IF NOT EXISTS idx_evolution_proposal_status ON knowledge_evolution_proposals (tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_evolution_proposal_question ON knowledge_evolution_proposals (tenant_id, knowledge_base_id, question_hash);

CREATE TABLE IF NOT EXISTS knowledge_version_links (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id TEXT NOT NULL,
    version_group_id TEXT NOT NULL,
    old_knowledge_id TEXT NOT NULL,
    new_knowledge_id TEXT NOT NULL,
    relation_type TEXT NOT NULL DEFAULT 'supersedes',
    active INTEGER NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_knowledge_version_link_new ON knowledge_version_links (tenant_id, new_knowledge_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_version_link_old ON knowledge_version_links (tenant_id, old_knowledge_id, active);

CREATE TABLE IF NOT EXISTS knowledge_regression_cases (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id TEXT NOT NULL,
    proposal_id TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL DEFAULT '',
    expected_answer TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT 'normal',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_regression_case_scope ON knowledge_regression_cases (tenant_id, knowledge_base_id, enabled);

CREATE TABLE IF NOT EXISTS knowledge_regression_runs (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id TEXT NOT NULL,
    proposal_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    total INTEGER NOT NULL DEFAULT 0,
    passed INTEGER NOT NULL DEFAULT 0,
    critical_total INTEGER NOT NULL DEFAULT 0,
    critical_passed INTEGER NOT NULL DEFAULT 0,
    failure_summary TEXT NOT NULL DEFAULT '',
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_regression_run_scope ON knowledge_regression_runs (tenant_id, proposal_id, started_at DESC);

CREATE TABLE IF NOT EXISTS knowledge_regression_results (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    case_id TEXT NOT NULL,
    answer TEXT NOT NULL DEFAULT '',
    retrieved_knowledge_ids TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'pending',
    failure_reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_regression_result_run ON knowledge_regression_results (run_id, status);
