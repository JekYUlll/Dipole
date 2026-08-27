ALTER TABLE agent_event_subscriptions
    ADD COLUMN created_by_uuid VARCHAR(24) NULL AFTER filter_json,
    ADD COLUMN revoked_by_uuid VARCHAR(24) NULL AFTER revoked_at,
    ADD COLUMN revoke_reason VARCHAR(1000) NULL AFTER revoked_by_uuid,
    ADD COLUMN updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) AFTER created_at;

UPDATE agent_event_subscriptions AS s
JOIN agent_definition_versions AS d
  ON d.definition_uuid = s.definition_uuid AND d.version = s.definition_version
SET s.created_by_uuid = d.owner_uuid,
    s.revoked_by_uuid = CASE WHEN s.status = 'revoked' THEN d.owner_uuid ELSE NULL END,
    s.revoke_reason = CASE WHEN s.status = 'revoked' THEN 'legacy_v28_migration' ELSE NULL END,
    s.updated_at = COALESCE(s.revoked_at, s.created_at);

ALTER TABLE agent_event_subscriptions
    MODIFY COLUMN created_by_uuid VARCHAR(24) NOT NULL,
    ADD KEY idx_agent_event_subscription_owner (tenant_id, created_by_uuid, subscription_uuid),
    ADD CONSTRAINT chk_agent_event_subscription_audit CHECK (
        (status = 'active' AND revoked_at IS NULL AND revoked_by_uuid IS NULL AND revoke_reason IS NULL) OR
        (status = 'revoked' AND revoked_at IS NOT NULL AND revoked_by_uuid IS NOT NULL AND revoke_reason IS NOT NULL)
    );
