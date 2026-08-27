CREATE TABLE IF NOT EXISTS agent_tool_invocations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    invocation_uuid VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    principal_uuid VARCHAR(64) NOT NULL,
    agent_uuid VARCHAR(24) NOT NULL,
    task_uuid VARCHAR(64) NOT NULL,
    run_uuid VARCHAR(64) NOT NULL,
    transport VARCHAR(16) NOT NULL,
    tool_name VARCHAR(64) NOT NULL,
    capability_id VARCHAR(128) NOT NULL,
    arguments_sha256 CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL,
    result_sha256 CHAR(64) NULL,
    result_bytes BIGINT UNSIGNED NULL,
    latency_ms BIGINT UNSIGNED NULL,
    error_code VARCHAR(64) NULL,
    request_id VARCHAR(128) NULL,
    trace_id VARCHAR(128) NULL,
    started_at DATETIME(3) NOT NULL,
    finished_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_tool_invocation_uuid (invocation_uuid),
    KEY idx_agent_tool_invocation_task_run (task_uuid, run_uuid, status),
    CONSTRAINT fk_agent_tool_invocation_task FOREIGN KEY (task_uuid) REFERENCES agent_tasks(task_uuid),
    CONSTRAINT fk_agent_tool_invocation_run FOREIGN KEY (run_uuid) REFERENCES agent_runs(run_uuid),
    CONSTRAINT chk_agent_tool_invocation_transport CHECK (transport = 'mcp'),
    CONSTRAINT chk_agent_tool_invocation_status CHECK (status IN ('running', 'completed', 'failed')),
    CONSTRAINT chk_agent_tool_invocation_terminal CHECK (
        (status = 'running' AND result_sha256 IS NULL AND result_bytes IS NULL AND latency_ms IS NULL AND error_code IS NULL AND finished_at IS NULL)
        OR (status = 'completed' AND result_sha256 IS NOT NULL AND result_bytes IS NOT NULL AND latency_ms IS NOT NULL AND error_code IS NULL AND finished_at IS NOT NULL)
        OR (status = 'failed' AND result_sha256 IS NULL AND result_bytes IS NULL AND latency_ms IS NOT NULL AND error_code IS NOT NULL AND finished_at IS NOT NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
