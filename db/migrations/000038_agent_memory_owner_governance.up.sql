ALTER TABLE agent_memories
    ADD COLUMN revoked_by_uuid VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN revoke_reason VARCHAR(1000) NOT NULL DEFAULT '';

UPDATE agent_memories
SET revoked_by_uuid = principal_uuid,
    revoke_reason = 'legacy internal revocation'
WHERE status = 'revoked';

ALTER TABLE agent_memories
    ADD CONSTRAINT chk_agent_memory_revocation_audit CHECK (
        (status = 'active' AND revoked_by_uuid = '' AND revoke_reason = '') OR
        (status = 'revoked' AND revoked_by_uuid <> '' AND revoke_reason <> '')
    );
