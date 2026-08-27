-- name: InsertAgentDefinitionVersion :exec
INSERT INTO agent_definition_versions (
    definition_uuid, version, tenant_id, owner_uuid, agent_uuid, status,
    permissions_json, scopes_json, valid_from, expires_at, revoked_at,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: GetLatestAgentDefinition :one
SELECT * FROM agent_definition_versions
WHERE tenant_id = ? AND agent_uuid = ?
ORDER BY version DESC
LIMIT 1;

-- name: GetAgentDefinitionVersion :one
SELECT * FROM agent_definition_versions
WHERE definition_uuid = ? AND version = ?
LIMIT 1;

-- name: RevokeAgentDefinitionVersion :execrows
UPDATE agent_definition_versions
SET status = 'revoked', revoked_at = ?, updated_at = NOW(3)
WHERE definition_uuid = ? AND version = ? AND revoked_at IS NULL;

-- name: InsertAgentTask :execrows
INSERT IGNORE INTO agent_tasks (
    task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid,
    agent_uuid, status, trigger_type, trigger_ref, goal, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: GetAgentTask :one
SELECT * FROM agent_tasks WHERE task_uuid = ? LIMIT 1;

-- name: TransitionAgentTaskStatus :execrows
UPDATE agent_tasks
SET status = ?, updated_at = NOW(3)
WHERE task_uuid = ? AND status = ?;

-- name: InsertAgentApproval :exec
INSERT INTO agent_approvals (
    approval_uuid, task_uuid, capability_id, resource_scope_json, scope_sha256,
    arguments_sha256, nonce_sha256, status, approved_by_uuid, expires_at,
    consumed_at, revoked_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: ConsumeAgentApproval :execrows
UPDATE agent_approvals
SET status = 'consumed', consumed_at = ?, updated_at = NOW(3)
WHERE approval_uuid = ?
  AND task_uuid = ?
  AND capability_id = ?
  AND scope_sha256 = ?
  AND arguments_sha256 = ?
  AND nonce_sha256 = ?
  AND status = 'approved'
  AND consumed_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > ?;

-- name: ApproveAgentApproval :execrows
UPDATE agent_approvals
SET status = 'approved', approved_by_uuid = ?, updated_at = NOW(3)
WHERE approval_uuid = ?
  AND status = 'pending'
  AND consumed_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > ?;

-- name: RevokeAgentApproval :execrows
UPDATE agent_approvals
SET status = 'revoked', revoked_at = ?, updated_at = NOW(3)
WHERE approval_uuid = ? AND consumed_at IS NULL AND revoked_at IS NULL;
