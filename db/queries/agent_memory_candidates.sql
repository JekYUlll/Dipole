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
