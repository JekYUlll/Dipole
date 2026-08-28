-- name: InsertAgentMCPToolRound :execrows
INSERT IGNORE INTO agent_mcp_tool_rounds (
    round_uuid, invocation_uuid, task_uuid, run_uuid, round_number,
    request_sha256, owner_token_sha256, status
) VALUES (?, ?, ?, ?, ?, ?, ?, 'executing');

-- name: GetAgentMCPToolRound :one
SELECT * FROM agent_mcp_tool_rounds WHERE round_uuid = ? LIMIT 1;

-- name: FinishAgentMCPToolRound :execrows
UPDATE agent_mcp_tool_rounds
SET status = ?,
    result_json = ?,
    result_sha256 = ?,
    result_bytes = ?,
    error_code = ?,
    finished_at = CURRENT_TIMESTAMP(3)
WHERE round_uuid = ?
  AND owner_token_sha256 = ?
  AND status = 'executing';
