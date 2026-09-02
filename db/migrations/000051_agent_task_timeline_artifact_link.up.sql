ALTER TABLE agent_task_timeline_events
    ADD COLUMN artifact_uuid CHAR(64) NULL AFTER approval_uuid,
    ADD KEY idx_agent_task_timeline_artifact (artifact_uuid),
    ADD CONSTRAINT fk_agent_task_timeline_artifact
        FOREIGN KEY (artifact_uuid) REFERENCES agent_artifacts(artifact_uuid) ON DELETE RESTRICT;

ALTER TABLE agent_task_timeline_repairs
    ADD COLUMN artifact_uuid CHAR(64) NULL AFTER approval_uuid,
    ADD KEY idx_agent_timeline_repairs_artifact (artifact_uuid),
    ADD CONSTRAINT fk_agent_timeline_repair_artifact
        FOREIGN KEY (artifact_uuid) REFERENCES agent_artifacts(artifact_uuid) ON DELETE RESTRICT;
