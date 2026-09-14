-- Refuse rollback if a task has multiple stages; never discard audit history.
ALTER TABLE agent_model_runs
    DROP INDEX idx_agent_model_runs_task_stage,
    ADD UNIQUE KEY idx_agent_model_runs_task (task_uuid),
    DROP COLUMN stage;
