-- name: InsertAgentToolInvocation :execrows
INSERT IGNORE INTO agent_tool_invocations (
    invocation_uuid, tenant_id, principal_uuid, agent_uuid, task_uuid, run_uuid,
    transport, tool_name, capability_id, arguments_sha256, status,
    request_id, trace_id, started_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FinishAgentToolInvocation :execrows
UPDATE agent_tool_invocations
SET status = ?,
    result_sha256 = ?,
    result_bytes = ?,
    latency_ms = ?,
    error_code = ?,
    finished_at = CURRENT_TIMESTAMP(3)
WHERE invocation_uuid = ?
  AND task_uuid = ?
  AND run_uuid = ?
  AND status = 'running';
