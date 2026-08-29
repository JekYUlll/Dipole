ALTER TABLE agent_workflow_repair_operator_grants
    ADD COLUMN grant_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER user_uuid,
    ADD COLUMN can_execute BOOLEAN NOT NULL DEFAULT FALSE AFTER can_approve;
