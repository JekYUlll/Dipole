ALTER TABLE agent_runs
    DROP INDEX idx_agent_runs_task_runtime_mode_attempt_desc,
    DROP INDEX idx_agent_runs_task_runtime_mode_attempt,
    DROP COLUMN attempt,
    ADD UNIQUE KEY idx_agent_runs_task_runtime_mode (task_uuid, runtime_id, mode);
