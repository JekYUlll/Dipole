ALTER TABLE agent_memory_candidates
    ADD COLUMN promoted_memory_uuid VARCHAR(64) NULL AFTER status,
    ADD COLUMN promoted_at DATETIME(3) NULL AFTER promoted_memory_uuid,
    ADD UNIQUE KEY uk_agent_memory_candidate_promoted_memory (promoted_memory_uuid),
    ADD CONSTRAINT fk_agent_memory_candidate_promoted_memory FOREIGN KEY (promoted_memory_uuid) REFERENCES agent_memories(memory_uuid) ON DELETE RESTRICT;
