ALTER TABLE agent_memories
    DROP CHECK chk_agent_memory_revocation_audit,
    DROP COLUMN revoke_reason,
    DROP COLUMN revoked_by_uuid;
