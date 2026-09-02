ALTER TABLE agent_shadow_steps
    ADD COLUMN authorization_resource_type VARCHAR(32) NULL AFTER capability_id,
    ADD COLUMN authorization_resource_id VARCHAR(256) NULL AFTER authorization_resource_type,
    ADD COLUMN authorization_action VARCHAR(64) NULL AFTER authorization_resource_id,
    ADD COLUMN authorization_decision VARCHAR(8) NULL AFTER authorization_action,
    ADD CONSTRAINT chk_agent_shadow_steps_authorization CHECK (
        (authorization_resource_type IS NULL AND authorization_resource_id IS NULL AND authorization_action IS NULL AND authorization_decision IS NULL)
        OR (authorization_resource_type IS NOT NULL AND authorization_resource_id IS NOT NULL AND authorization_action IS NOT NULL AND authorization_decision IN ('allowed', 'denied'))
    );
