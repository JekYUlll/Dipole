CREATE TABLE IF NOT EXISTS agent_oauth_token_lifecycle_maintenance_leases (
    handoff_uuid CHAR(64) NOT NULL,
    runtime_key_id VARCHAR(128) NOT NULL,
    lease_owner VARCHAR(128) NOT NULL,
    lease_generation BIGINT UNSIGNED NOT NULL,
    lease_expires_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (handoff_uuid),
    KEY idx_agent_oauth_lifecycle_maintenance_expiry (lease_expires_at),
    CONSTRAINT chk_agent_oauth_lifecycle_maintenance_generation CHECK (lease_generation > 0),
    CONSTRAINT fk_agent_oauth_lifecycle_maintenance_handoff
        FOREIGN KEY (handoff_uuid) REFERENCES agent_oauth_token_lifecycles(handoff_uuid)
        ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
