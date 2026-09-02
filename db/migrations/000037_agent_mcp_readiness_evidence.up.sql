CREATE TABLE IF NOT EXISTS agent_mcp_readiness_evidence (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    evidence_uuid CHAR(64) NOT NULL,
    schema_version VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    profile_binding_sha256 CHAR(64) NOT NULL,
    runtime_binding_sha256 CHAR(64) NOT NULL,
    content_json TEXT NOT NULL,
    content_sha256 CHAR(64) NOT NULL,
    operator_uuid VARCHAR(64) NOT NULL,
    request_id VARCHAR(128) NULL,
    trace_id VARCHAR(128) NULL,
    status ENUM('recorded') NOT NULL DEFAULT 'recorded',
    collected_at DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_mcp_readiness_evidence_uuid (evidence_uuid),
    KEY idx_agent_mcp_readiness_evidence_fresh (
        tenant_id, profile_binding_sha256, runtime_binding_sha256, expires_at, collected_at
    ),
    CONSTRAINT chk_agent_mcp_readiness_evidence_schema CHECK (
        schema_version = 'dipole.agent.external-mcp-readiness-evidence-record.v1'
    ),
    CONSTRAINT chk_agent_mcp_readiness_evidence_content CHECK (JSON_VALID(content_json)),
    CONSTRAINT chk_agent_mcp_readiness_evidence_window CHECK (collected_at < expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
