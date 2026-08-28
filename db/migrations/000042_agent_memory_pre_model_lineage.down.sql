DELETE lineage
FROM agent_memory_task_lineage AS lineage
LEFT JOIN agent_shadow_plans AS plans ON plans.task_uuid = lineage.task_uuid
WHERE plans.task_uuid IS NULL;

UPDATE agent_memory_task_lineage
SET source = 'runtime_write'
WHERE source = 'context_pre_model';

ALTER TABLE agent_memory_task_lineage
    DROP FOREIGN KEY fk_agent_memory_task_lineage_task,
    DROP CHECK chk_agent_memory_task_lineage_source,
    ADD CONSTRAINT fk_agent_memory_task_lineage_plan FOREIGN KEY (task_uuid) REFERENCES agent_shadow_plans(task_uuid) ON DELETE CASCADE,
    ADD CONSTRAINT chk_agent_memory_task_lineage_source CHECK (source IN ('context_manifest_backfill', 'runtime_write'));
