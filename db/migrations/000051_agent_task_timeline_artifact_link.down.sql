ALTER TABLE agent_task_timeline_repairs
    DROP FOREIGN KEY fk_agent_timeline_repair_artifact,
    DROP KEY idx_agent_timeline_repairs_artifact,
    DROP COLUMN artifact_uuid;

ALTER TABLE agent_task_timeline_events
    DROP FOREIGN KEY fk_agent_task_timeline_artifact,
    DROP KEY idx_agent_task_timeline_artifact,
    DROP COLUMN artifact_uuid;
