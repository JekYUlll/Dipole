-- name: GetAgentMemoryCandidateForPromotion :one
SELECT *
FROM agent_memory_candidates
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND candidate_uuid = sqlc.arg(candidate_uuid)
LIMIT 1
FOR UPDATE;

-- name: GetAgentMemoryCandidateReviewForPromotion :one
SELECT *
FROM agent_memory_candidate_reviews
WHERE candidate_uuid = sqlc.arg(candidate_uuid)
  AND review_uuid = sqlc.arg(review_uuid)
LIMIT 1;

-- name: ListOwnedAgentMemoryCandidates :many
SELECT c.*, r.review_uuid, r.decision AS review_decision, r.reviewed_at
FROM agent_memory_candidates AS c
LEFT JOIN agent_memory_candidate_reviews AS r
  ON r.candidate_uuid = c.candidate_uuid
 AND r.reviewer_uuid = c.principal_uuid
 AND r.decision = 'accepted'
WHERE c.tenant_id = sqlc.arg(tenant_id)
  AND c.principal_uuid = sqlc.arg(principal_uuid)
  AND c.candidate_uuid > sqlc.arg(after_candidate_uuid)
ORDER BY c.candidate_uuid ASC, r.reviewed_at DESC
LIMIT ?;

-- name: PromoteAgentMemoryCandidate :execrows
UPDATE agent_memory_candidates
SET promoted_memory_uuid = sqlc.arg(promoted_memory_uuid),
    promoted_at = sqlc.arg(promoted_at)
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND candidate_uuid = sqlc.arg(candidate_uuid)
  AND candidate_sha256 = sqlc.arg(candidate_sha256)
  AND status = 'accepted'
  AND promoted_memory_uuid IS NULL;
