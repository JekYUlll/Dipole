-- name: GetAgentRuntimePromotionOperatorGrant :one
SELECT * FROM agent_runtime_promotion_operator_grants
WHERE tenant_id = ? AND user_uuid = ? LIMIT 1;

-- name: InsertAgentRuntimePromotionProposal :execrows
INSERT IGNORE INTO agent_runtime_promotion_proposals (
    proposal_uuid, tenant_id, runtime_id, candidate_version, definition_uuid, definition_version,
    evidence_artifact_uuid, evidence_sha256, eval_suite_sha256, proposer_uuid, ticket_ref, reason,
    status, proposed_at, expires_at, grant_valid_from, grant_expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'proposed', ?, ?, ?, ?);

-- name: GetAgentRuntimePromotionProposal :one
SELECT * FROM agent_runtime_promotion_proposals WHERE proposal_uuid = ? LIMIT 1;

-- name: GetAgentRuntimePromotionProposalForUpdate :one
SELECT * FROM agent_runtime_promotion_proposals WHERE proposal_uuid = ? LIMIT 1 FOR UPDATE;

-- name: InsertAgentRuntimePromotionReview :execrows
INSERT IGNORE INTO agent_runtime_promotion_reviews (proposal_uuid, reviewer_uuid, decision, decided_at)
VALUES (?, ?, ?, ?);

-- name: GetAgentRuntimePromotionReview :one
SELECT * FROM agent_runtime_promotion_reviews WHERE proposal_uuid = ? LIMIT 1;

-- name: ApproveAgentRuntimePromotionProposal :execrows
UPDATE agent_runtime_promotion_proposals
SET status = 'approved', grant_uuid = ?, decided_at = ?, updated_at = UTC_TIMESTAMP()
WHERE proposal_uuid = ? AND status = 'proposed' AND expires_at > ?;

-- name: RejectAgentRuntimePromotionProposal :execrows
UPDATE agent_runtime_promotion_proposals
SET status = 'rejected', decided_at = ?, updated_at = UTC_TIMESTAMP()
WHERE proposal_uuid = ? AND status = 'proposed' AND expires_at > ?;

-- name: GetAgentRuntimePromotionGrantForUpdate :one
SELECT * FROM agent_runtime_promotion_grants WHERE grant_uuid = ? LIMIT 1 FOR UPDATE;

-- name: InsertAgentRuntimePromotionRevocation :execrows
INSERT IGNORE INTO agent_runtime_promotion_revocations (
    grant_uuid, tenant_id, revoked_by_uuid, ticket_ref, reason, revoked_at
) VALUES (?, ?, ?, ?, ?, ?);

-- name: GetAgentRuntimePromotionRevocation :one
SELECT * FROM agent_runtime_promotion_revocations WHERE grant_uuid = ? LIMIT 1;
