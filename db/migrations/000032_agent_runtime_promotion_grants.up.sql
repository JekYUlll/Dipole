CREATE TABLE IF NOT EXISTS agent_runtime_promotion_grants (
    grant_uuid VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    runtime_id VARCHAR(64) NOT NULL,
    candidate_version VARCHAR(128) NOT NULL,
    definition_uuid VARCHAR(64) NOT NULL,
    definition_version BIGINT UNSIGNED NOT NULL,
    policy_version VARCHAR(64) NOT NULL,
    evidence_sha256 CHAR(64) NOT NULL,
    eval_suite_sha256 CHAR(64) NOT NULL,
    granted_by_uuid VARCHAR(24) NOT NULL,
    reviewed_by_uuid VARCHAR(24) NOT NULL,
    valid_from DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (grant_uuid),
    UNIQUE KEY idx_agent_runtime_promotion_binding (
        tenant_id, runtime_id, candidate_version, definition_uuid, definition_version
    ),
    KEY idx_agent_runtime_promotion_active (runtime_id, revoked_at, expires_at),
    CONSTRAINT fk_agent_runtime_promotion_definition
        FOREIGN KEY (definition_uuid, definition_version)
        REFERENCES agent_definition_versions (definition_uuid, version),
    CONSTRAINT chk_agent_runtime_promotion_policy
        CHECK (policy_version = 'dipole.agent.shadow-promotion-policy.v2'),
    CONSTRAINT chk_agent_runtime_promotion_review
        CHECK (granted_by_uuid <> reviewed_by_uuid),
    CONSTRAINT chk_agent_runtime_promotion_window
        CHECK (valid_from < expires_at),
    CONSTRAINT chk_agent_runtime_promotion_revocation
        CHECK (revoked_at IS NULL OR revoked_at >= valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE agent_runs
    ADD COLUMN candidate_version VARCHAR(128) NULL AFTER runtime_id;
