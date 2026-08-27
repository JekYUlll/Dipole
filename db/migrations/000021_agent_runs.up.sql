CREATE TABLE IF NOT EXISTS agent_runs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    run_uuid VARCHAR(64) NOT NULL,
    task_uuid VARCHAR(64) NOT NULL,
    runtime_id VARCHAR(64) NOT NULL,
    mode VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL,
    started_at DATETIME(3) NOT NULL,
    completed_at DATETIME(3) NULL,
    last_error TEXT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY idx_agent_runs_uuid (run_uuid),
    UNIQUE KEY idx_agent_runs_task_runtime_mode (task_uuid, runtime_id, mode),
    KEY idx_agent_runs_status (status, updated_at),
    CONSTRAINT fk_agent_runs_task FOREIGN KEY (task_uuid) REFERENCES agent_tasks(task_uuid),
    CONSTRAINT chk_agent_runs_mode CHECK (mode IN ('embedded', 'shadow', 'active')),
    CONSTRAINT chk_agent_runs_status CHECK (status IN ('running', 'completed', 'failed', 'cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE agent_shadow_steps
    ADD COLUMN claim_token CHAR(36) NULL AFTER attempt_count,
    ADD COLUMN lease_expires_at DATETIME(3) NULL AFTER claim_token;
