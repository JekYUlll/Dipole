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

-- name: InsertAgentEventSubscription :exec
INSERT INTO agent_event_subscriptions (
    subscription_uuid, definition_uuid, definition_version, tenant_id, agent_uuid,
    status, event_type, resource_type, resource_id, filter_kind, filter_json,
    created_at, revoked_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), ?);

-- name: ListMatchingAgentEventSubscriptions :many
SELECT s.*
FROM agent_event_subscriptions AS s
JOIN agent_definition_versions AS d
  ON d.definition_uuid = s.definition_uuid AND d.version = s.definition_version
WHERE s.tenant_id = ? AND s.agent_uuid = ? AND s.event_type = ?
  AND s.resource_type = ? AND (s.resource_id = ? OR s.resource_id = '*')
  AND s.status = 'active' AND s.revoked_at IS NULL
  AND d.status = 'active' AND d.revoked_at IS NULL
ORDER BY s.subscription_uuid ASC;

-- name: GetAgentEventSubscription :one
SELECT * FROM agent_event_subscriptions
WHERE subscription_uuid = ?
LIMIT 1;

-- name: RevokeAgentEventSubscription :execrows
UPDATE agent_event_subscriptions
SET status = 'revoked', revoked_at = ?
WHERE subscription_uuid = ? AND status = 'active' AND revoked_at IS NULL;

-- name: InsertAgentTask :execrows
INSERT IGNORE INTO agent_tasks (
    task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid,
    agent_uuid, status, trigger_type, trigger_ref, trigger_subscription_uuid, goal, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: GetAgentTask :one
SELECT * FROM agent_tasks WHERE task_uuid = ? LIMIT 1;

-- name: TransitionAgentTaskStatus :execrows
UPDATE agent_tasks
SET status = ?, updated_at = NOW(3)
WHERE task_uuid = ? AND status = ?;

-- name: ProjectAgentTaskWorkflowState :execrows
UPDATE agent_tasks
SET workflow_id = ?, workflow_run_id = ?, workflow_status = ?, workflow_revision = ?,
    workflow_updated_at = UTC_TIMESTAMP()
WHERE task_uuid = ?
  AND (workflow_id IS NULL OR (workflow_id = ? AND workflow_run_id = ?))
  AND (workflow_revision IS NULL OR workflow_revision < ?);

-- name: ListAgentTaskWorkflowProjectionSnapshots :many
SELECT t.task_uuid, t.workflow_id, t.workflow_run_id, t.workflow_status,
       t.workflow_revision, t.workflow_updated_at
FROM agent_tasks AS t
JOIN agent_runs AS r ON r.task_uuid = t.task_uuid
WHERE r.runtime_id = ? AND r.mode = ? AND t.task_uuid > ?
ORDER BY t.task_uuid ASC
LIMIT ?;

-- name: InsertAgentRun :execrows
INSERT INTO agent_runs (
    run_uuid, task_uuid, runtime_id, mode, status, started_at
) VALUES (?, ?, ?, ?, 'running', UTC_TIMESTAMP());

-- name: GetAgentRun :one
SELECT * FROM agent_runs WHERE run_uuid = ? LIMIT 1;

-- name: TransitionAgentRunStatus :execrows
UPDATE agent_runs
SET status = ?, completed_at = UTC_TIMESTAMP(), last_error = ?
WHERE run_uuid = ? AND status = ?;

-- name: InsertAgentApproval :exec
INSERT INTO agent_approvals (
    approval_uuid, task_uuid, capability_id, resource_scope_json, scope_sha256,
    arguments_sha256, nonce_sha256, status, approved_by_uuid, expires_at,
    consumed_at, revoked_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: GetAgentApproval :one
SELECT * FROM agent_approvals WHERE approval_uuid = ? LIMIT 1;

-- name: ListApprovedAgentApprovalGrants :many
SELECT *
FROM agent_approvals
WHERE task_uuid = ?
  AND capability_id = ?
  AND scope_sha256 = ?
  AND arguments_sha256 = ?
  AND status = 'approved'
  AND consumed_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > ?
ORDER BY id
LIMIT ?;

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

-- name: DenyAgentApproval :execrows
UPDATE agent_approvals
SET status = 'revoked', revoked_at = ?, updated_at = NOW(3)
WHERE approval_uuid = ? AND status = 'pending' AND consumed_at IS NULL AND revoked_at IS NULL;

-- name: GetAgentWorkflowRepairOperatorGrant :one
SELECT * FROM agent_workflow_repair_operator_grants WHERE user_uuid = ? LIMIT 1;

-- name: InsertAgentWorkflowRepairProposal :execrows
INSERT IGNORE INTO agent_workflow_repair_proposals (
    proposal_uuid, task_uuid, outcome, action, proposer_uuid, ticket_ref, reason,
    projected_json, temporal_json, evidence_sha256, status, required_approvals, proposed_at, expires_at
) VALUES (?, ?, ?, 'reproject_from_temporal', ?, ?, ?, ?, ?, ?, 'proposed', 2, ?, ?);

-- name: GetAgentWorkflowRepairProposal :one
SELECT * FROM agent_workflow_repair_proposals WHERE proposal_uuid = ? LIMIT 1;

-- name: InsertAgentWorkflowRepairDecision :execrows
INSERT IGNORE INTO agent_workflow_repair_decisions (
    proposal_uuid, approver_uuid, decision, decided_at
)
SELECT ?, ?, ?, UTC_TIMESTAMP()
FROM agent_workflow_repair_proposals AS p
WHERE p.proposal_uuid = ? AND p.proposer_uuid <> ? AND p.status = 'proposed' AND p.expires_at > UTC_TIMESTAMP();

-- name: GetAgentWorkflowRepairDecision :one
SELECT * FROM agent_workflow_repair_decisions WHERE proposal_uuid = ? AND approver_uuid = ? LIMIT 1;

-- name: CountAgentWorkflowRepairDecisions :one
SELECT
    CAST(SUM(decision = 'approved') AS UNSIGNED) AS approved_count,
    CAST(SUM(decision = 'rejected') AS UNSIGNED) AS rejected_count
FROM agent_workflow_repair_decisions
WHERE proposal_uuid = ?;

-- name: ApproveAgentWorkflowRepairProposal :execrows
UPDATE agent_workflow_repair_proposals AS p
SET p.status = 'approved', p.decided_at = UTC_TIMESTAMP()
WHERE p.proposal_uuid = ? AND p.status = 'proposed' AND p.expires_at > UTC_TIMESTAMP()
  AND (SELECT COUNT(*) FROM agent_workflow_repair_decisions AS d WHERE d.proposal_uuid = p.proposal_uuid AND d.decision = 'approved') >= p.required_approvals
  AND NOT EXISTS (SELECT 1 FROM agent_workflow_repair_decisions AS d WHERE d.proposal_uuid = p.proposal_uuid AND d.decision = 'rejected');

-- name: RejectAgentWorkflowRepairProposal :execrows
UPDATE agent_workflow_repair_proposals AS p
SET p.status = 'rejected', p.decided_at = UTC_TIMESTAMP()
WHERE p.proposal_uuid = ? AND p.status = 'proposed'
  AND EXISTS (SELECT 1 FROM agent_workflow_repair_decisions AS d WHERE d.proposal_uuid = p.proposal_uuid AND d.decision = 'rejected');

-- name: ExpireAgentWorkflowRepairProposal :execrows
UPDATE agent_workflow_repair_proposals
SET status = 'expired', decided_at = UTC_TIMESTAMP()
WHERE proposal_uuid = ? AND status = 'proposed' AND expires_at <= UTC_TIMESTAMP();
