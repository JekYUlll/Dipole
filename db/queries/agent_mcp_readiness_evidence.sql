-- name: InsertAgentMCPReadinessEvidence :execrows
INSERT IGNORE INTO agent_mcp_readiness_evidence (
    evidence_uuid, schema_version, tenant_id, profile_binding_sha256, runtime_binding_sha256,
    content_json, content_sha256, operator_uuid, request_id, trace_id, status, collected_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'recorded', ?, ?);

-- name: GetAgentMCPReadinessEvidence :one
SELECT * FROM agent_mcp_readiness_evidence
WHERE tenant_id = ? AND evidence_uuid = ?
LIMIT 1;

-- name: GetFreshAgentMCPReadinessEvidence :one
SELECT * FROM agent_mcp_readiness_evidence
WHERE tenant_id = ?
  AND profile_binding_sha256 = ?
  AND runtime_binding_sha256 = ?
  AND status = 'recorded'
  AND expires_at > ?
  AND collected_at <= ?
ORDER BY collected_at DESC, id DESC
LIMIT 1;
