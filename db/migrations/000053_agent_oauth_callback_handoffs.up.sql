CREATE TABLE IF NOT EXISTS agent_oauth_callback_handoffs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    handoff_uuid CHAR(64) NOT NULL,
    transaction_uuid CHAR(64) NOT NULL,
    owner_user_uuid VARCHAR(64) NOT NULL,
    issuer VARCHAR(2048) NOT NULL,
    redirect_uri VARCHAR(2048) NOT NULL,
    authorization_code_sha256 CHAR(64) NOT NULL,
    sealed_authorization_code TEXT NOT NULL,
    runtime_key_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    lease_owner VARCHAR(128) NULL,
    lease_expires_at DATETIME(3) NULL,
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    expires_at DATETIME(3) NOT NULL,
    completed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_oauth_callback_handoff_uuid (handoff_uuid),
    UNIQUE KEY uk_agent_oauth_callback_handoff_transaction (transaction_uuid),
    KEY idx_agent_oauth_callback_handoff_claim (status, lease_expires_at, expires_at),
    CONSTRAINT chk_agent_oauth_callback_handoff_status CHECK (status IN ('callback_recorded', 'exchange_claimed', 'exchanged')),
    CONSTRAINT chk_agent_oauth_callback_handoff_expiry CHECK (expires_at > created_at),
    CONSTRAINT chk_agent_oauth_callback_handoff_lease CHECK (
        (status = 'callback_recorded' AND lease_owner IS NULL AND lease_expires_at IS NULL AND completed_at IS NULL)
        OR (status = 'exchange_claimed' AND lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL AND completed_at IS NULL)
        OR (status = 'exchanged' AND lease_owner IS NULL AND lease_expires_at IS NULL AND completed_at IS NOT NULL AND completed_at < expires_at)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
