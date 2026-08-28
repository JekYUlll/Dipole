-- name: InsertAgentToolInvocation :execrows
INSERT IGNORE INTO agent_tool_invocations (
    invocation_uuid, tenant_id, principal_uuid, agent_uuid, task_uuid, run_uuid,
    transport, tool_name, capability_id, arguments_sha256, profile_id, server_id, arguments_json, status,
    request_id, trace_id, approval_uuid, started_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAgentToolInvocation :one
SELECT
    id, invocation_uuid, tenant_id, principal_uuid, agent_uuid, task_uuid, run_uuid,
    transport, tool_name, capability_id, arguments_sha256, status, result_sha256,
    result_bytes, latency_ms, error_code, request_id, trace_id, started_at, finished_at,
    created_at, updated_at, approval_uuid, action_resource_type, action_resource_uuid,
    action_command_kind, action_command_id, profile_id, server_id,
    COALESCE(arguments_json, JSON_EXTRACT('null', '$')) AS arguments_json
FROM agent_tool_invocations
WHERE invocation_uuid = ?
LIMIT 1;

-- name: FinishAgentToolInvocation :execrows
UPDATE agent_tool_invocations
SET status = ?,
    result_sha256 = ?,
    result_bytes = ?,
    latency_ms = ?,
    error_code = ?,
    action_resource_type = ?,
    action_resource_uuid = ?,
    action_command_kind = ?,
    action_command_id = ?,
    finished_at = CURRENT_TIMESTAMP(3)
WHERE invocation_uuid = ?
  AND task_uuid = ?
  AND run_uuid = ?
  AND status = 'running';
