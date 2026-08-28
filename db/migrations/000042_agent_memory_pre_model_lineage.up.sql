ALTER TABLE agent_memory_task_lineage
    DROP FOREIGN KEY fk_agent_memory_task_lineage_plan,
    DROP CHECK chk_agent_memory_task_lineage_source,
    ADD CONSTRAINT fk_agent_memory_task_lineage_task FOREIGN KEY (task_uuid) REFERENCES agent_tasks(task_uuid) ON DELETE CASCADE,
    ADD CONSTRAINT chk_agent_memory_task_lineage_source CHECK (source IN ('context_manifest_backfill', 'runtime_write', 'context_pre_model'));
