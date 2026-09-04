CREATE TABLE IF NOT EXISTS agent_oauth_token_lifecycles (
    handoff_uuid CHAR(64) NOT NULL,
    runtime_key_id VARCHAR(128) NOT NULL,
    state VARCHAR(32) NOT NULL,
    sealed_token_bundle TEXT NULL,
    token_bundle_sha256 CHAR(64) NULL,
    access_token_expires_at DATETIME(3) NULL,
    scope VARCHAR(2048) NULL,
    refresh_count INT UNSIGNED NOT NULL DEFAULT 0,
    revocation_reason VARCHAR(512) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (handoff_uuid),
    KEY idx_agent_oauth_token_lifecycle_expiry (state, access_token_expires_at),
    CONSTRAINT chk_agent_oauth_token_lifecycle_state CHECK (state IN ('active', 'refreshed', 'revoked', 'expired')),
    CONSTRAINT chk_agent_oauth_token_lifecycle_material CHECK (
        (state IN ('active', 'refreshed')
            AND sealed_token_bundle IS NOT NULL
            AND token_bundle_sha256 IS NOT NULL
            AND access_token_expires_at IS NOT NULL
            AND revocation_reason IS NULL)
        OR (state = 'revoked'
            AND sealed_token_bundle IS NULL
            AND token_bundle_sha256 IS NULL
            AND access_token_expires_at IS NULL
            AND revocation_reason IS NOT NULL)
        OR (state = 'expired'
            AND sealed_token_bundle IS NULL
            AND token_bundle_sha256 IS NULL
            AND access_token_expires_at IS NULL
            AND revocation_reason IS NULL)
    ),
    CONSTRAINT fk_agent_oauth_token_lifecycle_handoff
        FOREIGN KEY (handoff_uuid) REFERENCES agent_oauth_callback_handoffs(handoff_uuid)
        ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
