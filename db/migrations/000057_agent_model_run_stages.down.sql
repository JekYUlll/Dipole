ALTER TABLE agent_model_runs
    DROP INDEX idx_agent_model_runs_task_stage,
    DROP COLUMN stage,
    ADD UNIQUE KEY idx_agent_model_runs_task (task_uuid);
