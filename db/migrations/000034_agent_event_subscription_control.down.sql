ALTER TABLE agent_event_subscriptions
    DROP CHECK chk_agent_event_subscription_audit,
    DROP INDEX idx_agent_event_subscription_owner,
    DROP COLUMN revoke_reason,
    DROP COLUMN revoked_by_uuid,
    DROP COLUMN updated_at,
    DROP COLUMN created_by_uuid;
