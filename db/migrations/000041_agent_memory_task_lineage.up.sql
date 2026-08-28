CREATE TABLE IF NOT EXISTS agent_memory_task_lineage (
    memory_uuid VARCHAR(64) NOT NULL,
    task_uuid VARCHAR(64) NOT NULL,
    representation VARCHAR(16) NOT NULL,
    source VARCHAR(32) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (memory_uuid, task_uuid),
    KEY idx_agent_memory_task_lineage_task (task_uuid, memory_uuid),
    CONSTRAINT fk_agent_memory_task_lineage_memory FOREIGN KEY (memory_uuid) REFERENCES agent_memories(memory_uuid) ON DELETE RESTRICT,
    CONSTRAINT fk_agent_memory_task_lineage_plan FOREIGN KEY (task_uuid) REFERENCES agent_shadow_plans(task_uuid) ON DELETE CASCADE,
    CONSTRAINT chk_agent_memory_task_lineage_representation CHECK (representation IN ('full', 'compact')),
    CONSTRAINT chk_agent_memory_task_lineage_source CHECK (source IN ('context_manifest_backfill', 'runtime_write'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
