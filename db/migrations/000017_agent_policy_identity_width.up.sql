ALTER TABLE agent_definition_versions
    MODIFY owner_uuid VARCHAR(24) NOT NULL AFTER tenant_id,
    MODIFY agent_uuid VARCHAR(24) NOT NULL AFTER owner_uuid;

ALTER TABLE agent_tasks
    MODIFY principal_uuid VARCHAR(24) NOT NULL AFTER tenant_id,
    MODIFY agent_uuid VARCHAR(24) NOT NULL AFTER principal_uuid;

ALTER TABLE agent_approvals
    MODIFY approved_by_uuid VARCHAR(24) NOT NULL DEFAULT '' AFTER status;
