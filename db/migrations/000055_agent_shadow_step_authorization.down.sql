ALTER TABLE agent_shadow_steps
    DROP CONSTRAINT chk_agent_shadow_steps_authorization,
    DROP COLUMN authorization_decision,
    DROP COLUMN authorization_action,
    DROP COLUMN authorization_resource_id,
    DROP COLUMN authorization_resource_type;
