ALTER TABLE agent_memories
    DROP CONSTRAINT chk_agent_memory_content_erasure,
    DROP COLUMN content_erasure_reason_code,
    DROP COLUMN content_erased_by_uuid,
    DROP COLUMN content_erased_at;
