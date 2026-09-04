CREATE TABLE IF NOT EXISTS agent_runtime_promotion_operator_grant_audits (
    audit_uuid CHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    user_uuid VARCHAR(24) NOT NULL,
    action ENUM('granted', 'revoked') NOT NULL,
    can_propose BOOLEAN NOT NULL,
    can_review BOOLEAN NOT NULL,
    can_revoke BOOLEAN NOT NULL,
    granted_by_uuid VARCHAR(24) NOT NULL,
    ticket_ref VARCHAR(128) NOT NULL,
    reason VARCHAR(1000) NOT NULL,
    expires_at DATETIME(3) NULL,
    occurred_at DATETIME(3) NOT NULL,
    PRIMARY KEY (audit_uuid),
    KEY idx_agent_runtime_promotion_operator_grant_audit_target (tenant_id, user_uuid, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
