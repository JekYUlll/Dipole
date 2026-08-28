ALTER TABLE agent_memories
    ADD COLUMN memory_root_uuid VARCHAR(64) NOT NULL DEFAULT '' AFTER revoke_reason,
    ADD COLUMN memory_version INT UNSIGNED NOT NULL DEFAULT 1 AFTER memory_root_uuid,
    ADD COLUMN supersedes_memory_uuid VARCHAR(64) NULL AFTER memory_version,
    ADD COLUMN corrected_by_uuid VARCHAR(64) NOT NULL DEFAULT '' AFTER supersedes_memory_uuid,
    ADD COLUMN correction_reason VARCHAR(1000) NOT NULL DEFAULT '' AFTER corrected_by_uuid;

UPDATE agent_memories
SET memory_root_uuid = memory_uuid
WHERE memory_root_uuid = '';

ALTER TABLE agent_memories
    ADD CONSTRAINT fk_agent_memories_supersedes
        FOREIGN KEY (supersedes_memory_uuid) REFERENCES agent_memories(memory_uuid) ON DELETE RESTRICT,
    ADD CONSTRAINT chk_agent_memories_lineage CHECK (
        (memory_version = 1
            AND memory_root_uuid = memory_uuid
            AND supersedes_memory_uuid IS NULL
            AND corrected_by_uuid = ''
            AND correction_reason = '')
        OR
        (memory_version > 1
            AND memory_root_uuid <> ''
            AND supersedes_memory_uuid IS NOT NULL
            AND corrected_by_uuid <> ''
            AND correction_reason <> ''
            AND correction_reason NOT REGEXP '[[:cntrl:]]'
            AND source_type = 'owner_correction'
            AND source_id = supersedes_memory_uuid
            AND source_sequence REGEXP '^[1-9][0-9]*$'
            AND CAST(source_sequence AS UNSIGNED) = memory_version)
    ),
    ADD UNIQUE KEY uq_agent_memories_root_version (tenant_id, memory_root_uuid, memory_version),
    ADD UNIQUE KEY uq_agent_memories_supersedes (supersedes_memory_uuid);
