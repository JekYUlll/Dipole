CREATE TABLE IF NOT EXISTS agent_event_subscriptions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    subscription_uuid VARCHAR(64) NOT NULL,
    definition_uuid VARCHAR(64) NOT NULL,
    definition_version BIGINT UNSIGNED NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    agent_uuid VARCHAR(24) NOT NULL,
    status VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128) NOT NULL,
    filter_kind VARCHAR(32) NOT NULL,
    filter_json JSON NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_agent_event_subscription_uuid (subscription_uuid),
    KEY idx_agent_event_subscription_match (tenant_id, agent_uuid, event_type, resource_type, status, resource_id),
    CONSTRAINT chk_agent_event_subscription_status CHECK (status IN ('active', 'revoked')),
    CONSTRAINT chk_agent_event_subscription_filter CHECK (filter_kind IN ('all', 'message_contains_any')),
    CONSTRAINT fk_agent_event_subscription_definition
        FOREIGN KEY (definition_uuid, definition_version)
        REFERENCES agent_definition_versions (definition_uuid, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE agent_tasks
    ADD COLUMN trigger_subscription_uuid VARCHAR(64) NULL AFTER trigger_ref,
    ADD KEY idx_agent_task_subscription (trigger_subscription_uuid),
    ADD CONSTRAINT fk_agent_task_subscription
        FOREIGN KEY (trigger_subscription_uuid)
        REFERENCES agent_event_subscriptions (subscription_uuid);
