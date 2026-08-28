CREATE TABLE IF NOT EXISTS agent_mcp_tool_rounds (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    round_uuid CHAR(64) NOT NULL,
    invocation_uuid VARCHAR(64) NOT NULL,
    task_uuid VARCHAR(64) NOT NULL,
    run_uuid VARCHAR(64) NOT NULL,
    round_number TINYINT UNSIGNED NOT NULL,
    request_sha256 CHAR(64) NOT NULL,
    owner_token_sha256 CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL,
    result_json JSON NULL,
    result_sha256 CHAR(64) NULL,
    result_bytes BIGINT UNSIGNED NULL,
    error_code VARCHAR(64) NULL,
    claimed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    finished_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_mcp_tool_round_uuid (round_uuid),
    UNIQUE KEY uk_agent_mcp_tool_round_invocation_number (invocation_uuid, round_number),
    KEY idx_agent_mcp_tool_round_task_run (task_uuid, run_uuid, status),
    CONSTRAINT fk_agent_mcp_tool_round_invocation FOREIGN KEY (invocation_uuid) REFERENCES agent_tool_invocations(invocation_uuid),
    CONSTRAINT chk_agent_mcp_tool_round_number CHECK (round_number <= 1),
    CONSTRAINT chk_agent_mcp_tool_round_status CHECK (status IN ('executing', 'completed', 'failed')),
    CONSTRAINT chk_agent_mcp_tool_round_terminal CHECK (
        (status = 'executing' AND result_json IS NULL AND result_sha256 IS NULL AND result_bytes IS NULL AND error_code IS NULL AND finished_at IS NULL)
        OR (status = 'completed' AND result_json IS NOT NULL AND result_sha256 IS NOT NULL AND result_bytes IS NOT NULL AND error_code IS NULL AND finished_at IS NOT NULL)
        OR (status = 'failed' AND result_json IS NULL AND result_sha256 IS NULL AND result_bytes IS NULL AND error_code IS NOT NULL AND finished_at IS NOT NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
