ALTER TABLE agent_definition_versions
    DROP INDEX idx_agent_definition_tenant_agent_version,
    ADD UNIQUE KEY idx_agent_definition_tenant_owner_agent_version (tenant_id, owner_uuid, agent_uuid, version);
