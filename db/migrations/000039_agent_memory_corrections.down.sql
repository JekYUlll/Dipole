ALTER TABLE agent_memories
    DROP FOREIGN KEY fk_agent_memories_supersedes,
    DROP CHECK chk_agent_memories_lineage,
    DROP INDEX uq_agent_memories_root_version,
    DROP INDEX uq_agent_memories_supersedes,
    DROP COLUMN correction_reason,
    DROP COLUMN corrected_by_uuid,
    DROP COLUMN supersedes_memory_uuid,
    DROP COLUMN memory_version,
    DROP COLUMN memory_root_uuid;
