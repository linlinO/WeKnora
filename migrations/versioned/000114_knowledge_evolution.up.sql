CREATE TABLE IF NOT EXISTS knowledge_evolution_proposals (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    feedback_id VARCHAR(36) NOT NULL,
    question TEXT NOT NULL DEFAULT '',
    question_hash VARCHAR(64) NOT NULL DEFAULT '',
    original_answer TEXT NOT NULL DEFAULT '',
    candidate_answer TEXT NOT NULL DEFAULT '',
    similar_questions JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    conflict_status VARCHAR(20) NOT NULL DEFAULT 'none',
    conflict_summary TEXT NOT NULL DEFAULT '',
    conflict_knowledge_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    version_group_id VARCHAR(36) NOT NULL DEFAULT '',
    supersedes_knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    correction_knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    regression_run_id VARCHAR(36) NOT NULL DEFAULT '',
    auto_generated BOOLEAN NOT NULL DEFAULT FALSE,
    created_by VARCHAR(128) NOT NULL DEFAULT '',
    reviewer_id VARCHAR(128) NOT NULL DEFAULT '',
    review_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_evolution_proposal_feedback
    ON knowledge_evolution_proposals (tenant_id, feedback_id);
CREATE INDEX IF NOT EXISTS idx_evolution_proposal_status
    ON knowledge_evolution_proposals (tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_evolution_proposal_question
    ON knowledge_evolution_proposals (tenant_id, knowledge_base_id, question_hash);

CREATE TABLE IF NOT EXISTS knowledge_version_links (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    version_group_id VARCHAR(36) NOT NULL,
    old_knowledge_id VARCHAR(36) NOT NULL,
    new_knowledge_id VARCHAR(36) NOT NULL,
    relation_type VARCHAR(20) NOT NULL DEFAULT 'supersedes',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_knowledge_version_link_new
    ON knowledge_version_links (tenant_id, new_knowledge_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_version_link_old
    ON knowledge_version_links (tenant_id, old_knowledge_id, active);

CREATE TABLE IF NOT EXISTS knowledge_regression_cases (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    proposal_id VARCHAR(36) NOT NULL DEFAULT '',
    question TEXT NOT NULL DEFAULT '',
    expected_answer TEXT NOT NULL DEFAULT '',
    severity VARCHAR(20) NOT NULL DEFAULT 'normal',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_regression_case_scope
    ON knowledge_regression_cases (tenant_id, knowledge_base_id, enabled);

CREATE TABLE IF NOT EXISTS knowledge_regression_runs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    proposal_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total INTEGER NOT NULL DEFAULT 0,
    passed INTEGER NOT NULL DEFAULT 0,
    critical_total INTEGER NOT NULL DEFAULT 0,
    critical_passed INTEGER NOT NULL DEFAULT 0,
    failure_summary TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_regression_run_scope
    ON knowledge_regression_runs (tenant_id, proposal_id, started_at DESC);

CREATE TABLE IF NOT EXISTS knowledge_regression_results (
    id VARCHAR(36) PRIMARY KEY,
    run_id VARCHAR(36) NOT NULL,
    case_id VARCHAR(36) NOT NULL,
    answer TEXT NOT NULL DEFAULT '',
    retrieved_knowledge_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    failure_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_regression_result_run
    ON knowledge_regression_results (run_id, status);
