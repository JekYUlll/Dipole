CREATE TABLE IF NOT EXISTS agent_oauth_authorization_transactions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    transaction_uuid CHAR(64) NOT NULL,
    owner_user_uuid VARCHAR(64) NOT NULL,
    issuer VARCHAR(2048) NOT NULL,
    redirect_uri VARCHAR(2048) NOT NULL,
    state_sha256 CHAR(64) NOT NULL,
    sealed_code_verifier TEXT NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    consumed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_oauth_authorization_transaction_uuid (transaction_uuid),
    KEY idx_agent_oauth_authorization_transaction_consume (transaction_uuid, owner_user_uuid, state_sha256, consumed_at, expires_at),
    CONSTRAINT chk_agent_oauth_authorization_transaction_expiry CHECK (expires_at > created_at),
    CONSTRAINT chk_agent_oauth_authorization_transaction_consumed CHECK (consumed_at IS NULL OR consumed_at < expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
