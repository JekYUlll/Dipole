ALTER TABLE agent_runs
    ADD COLUMN attempt SMALLINT UNSIGNED NOT NULL DEFAULT 1 AFTER mode,
    DROP INDEX idx_agent_runs_task_runtime_mode,
    ADD UNIQUE KEY idx_agent_runs_task_runtime_mode_attempt (task_uuid, runtime_id, mode, attempt),
    ADD KEY idx_agent_runs_task_runtime_mode_attempt_desc (task_uuid, runtime_id, mode, attempt DESC);
