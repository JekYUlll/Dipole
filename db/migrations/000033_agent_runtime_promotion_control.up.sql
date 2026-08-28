ALTER TABLE agent_runtime_promotion_grants
    DROP CHECK chk_agent_runtime_promotion_revocation;

CREATE TABLE IF NOT EXISTS agent_runtime_promotion_operator_grants (
    tenant_id VARCHAR(64) NOT NULL,
    user_uuid VARCHAR(24) NOT NULL,
    can_propose BOOLEAN NOT NULL DEFAULT FALSE,
    can_review BOOLEAN NOT NULL DEFAULT FALSE,
    can_revoke BOOLEAN NOT NULL DEFAULT FALSE,
    granted_by_uuid VARCHAR(24) NOT NULL,
    valid_from DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NULL,
    revoked_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (tenant_id, user_uuid),
    KEY idx_agent_runtime_promotion_operator_active (user_uuid, revoked_at, expires_at),
    CONSTRAINT chk_agent_runtime_promotion_operator_role CHECK (can_propose OR can_review OR can_revoke),
    CONSTRAINT chk_agent_runtime_promotion_operator_window CHECK (expires_at IS NULL OR valid_from < expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_runtime_promotion_proposals (
    proposal_uuid CHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    runtime_id VARCHAR(64) NOT NULL,
    candidate_version VARCHAR(128) NOT NULL,
    definition_uuid VARCHAR(64) NOT NULL,
    definition_version BIGINT UNSIGNED NOT NULL,
    evidence_artifact_uuid CHAR(64) NOT NULL,
    evidence_sha256 CHAR(64) NOT NULL,
    eval_suite_sha256 CHAR(64) NOT NULL,
    proposer_uuid VARCHAR(24) NOT NULL,
    ticket_ref VARCHAR(128) NOT NULL,
    reason VARCHAR(1000) NOT NULL,
    status ENUM('proposed', 'approved', 'rejected') NOT NULL DEFAULT 'proposed',
    grant_uuid VARCHAR(64) NULL,
    proposed_at DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    grant_valid_from DATETIME(3) NOT NULL,
    grant_expires_at DATETIME(3) NOT NULL,
    decided_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (proposal_uuid),
    UNIQUE KEY idx_agent_runtime_promotion_proposal_grant (grant_uuid),
    KEY idx_agent_runtime_promotion_proposal_tenant (tenant_id, status, proposed_at),
    CONSTRAINT fk_agent_runtime_promotion_proposal_definition FOREIGN KEY (definition_uuid, definition_version)
        REFERENCES agent_definition_versions (definition_uuid, version),
    CONSTRAINT fk_agent_runtime_promotion_proposal_artifact FOREIGN KEY (evidence_artifact_uuid)
        REFERENCES agent_artifacts (artifact_uuid),
    CONSTRAINT fk_agent_runtime_promotion_proposal_grant FOREIGN KEY (grant_uuid)
        REFERENCES agent_runtime_promotion_grants (grant_uuid),
    CONSTRAINT chk_agent_runtime_promotion_proposal_window CHECK (proposed_at < expires_at),
    CONSTRAINT chk_agent_runtime_promotion_proposal_grant_window CHECK (grant_valid_from < grant_expires_at),
    CONSTRAINT chk_agent_runtime_promotion_proposal_result CHECK (
        (status = 'approved' AND grant_uuid IS NOT NULL AND decided_at IS NOT NULL) OR
        (status = 'rejected' AND grant_uuid IS NULL AND decided_at IS NOT NULL) OR
        (status = 'proposed' AND grant_uuid IS NULL AND decided_at IS NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_runtime_promotion_reviews (
    proposal_uuid CHAR(64) NOT NULL,
    reviewer_uuid VARCHAR(24) NOT NULL,
    decision ENUM('approved', 'rejected') NOT NULL,
    decided_at DATETIME(3) NOT NULL,
    PRIMARY KEY (proposal_uuid),
    CONSTRAINT fk_agent_runtime_promotion_review_proposal FOREIGN KEY (proposal_uuid)
        REFERENCES agent_runtime_promotion_proposals (proposal_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_runtime_promotion_revocations (
    grant_uuid VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    revoked_by_uuid VARCHAR(24) NOT NULL,
    ticket_ref VARCHAR(128) NOT NULL,
    reason VARCHAR(1000) NOT NULL,
    revoked_at DATETIME(3) NOT NULL,
    PRIMARY KEY (grant_uuid),
    KEY idx_agent_runtime_promotion_revocation_tenant (tenant_id, revoked_at),
    CONSTRAINT fk_agent_runtime_promotion_revocation_grant FOREIGN KEY (grant_uuid)
        REFERENCES agent_runtime_promotion_grants (grant_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
