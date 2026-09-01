ALTER TABLE agent_model_runs
    ADD COLUMN stage VARCHAR(32) NOT NULL DEFAULT 'plan' AFTER task_uuid,
    DROP INDEX idx_agent_model_runs_task,
    ADD UNIQUE KEY idx_agent_model_runs_task_stage (task_uuid, stage);
