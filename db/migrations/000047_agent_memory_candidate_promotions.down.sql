ALTER TABLE agent_memory_candidates
    DROP FOREIGN KEY fk_agent_memory_candidate_promoted_memory,
    DROP INDEX uk_agent_memory_candidate_promoted_memory,
    DROP COLUMN promoted_at,
    DROP COLUMN promoted_memory_uuid;
