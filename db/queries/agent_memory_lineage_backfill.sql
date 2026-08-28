-- name: GetAgentMemoryLineageBackfillHighWatermark :one
SELECT COALESCE(MAX(id), 0) AS high_watermark_id
FROM agent_shadow_plans
WHERE context_manifest_json IS NOT NULL;

-- name: ListAgentMemoryLineageBackfill :many
SELECT p.id AS plan_id, t.task_uuid, t.tenant_id, t.principal_uuid, p.context_manifest_json
FROM agent_shadow_plans AS p
JOIN agent_tasks AS t ON t.task_uuid = p.task_uuid
WHERE p.id > sqlc.arg(after_id)
  AND p.id <= sqlc.arg(through_id)
  AND p.context_manifest_json IS NOT NULL
ORDER BY p.id ASC
LIMIT ?;

-- name: GetAgentMemoryBackfillReference :one
SELECT memory_uuid
FROM agent_memories
WHERE memory_uuid = ? AND tenant_id = ? AND principal_uuid = ?
LIMIT 1;

-- name: UpsertAgentMemoryLineageBackfill :execresult
INSERT INTO agent_memory_task_lineage (memory_uuid, task_uuid, representation, source)
VALUES (?, ?, ?, 'context_manifest_backfill')
ON DUPLICATE KEY UPDATE
    representation = IF(representation = VALUES(representation), representation, NULL),
    source = IF(VALUES(source) = 'context_pre_model', 'context_pre_model', source);

-- name: EnsureAgentMemoryLineageBackfillJob :exec
INSERT INTO agent_memory_lineage_backfill_jobs (job_name, source_high_watermark_id, last_error)
VALUES (?, ?, '')
ON DUPLICATE KEY UPDATE job_name = VALUES(job_name);

-- name: LockAgentMemoryLineageBackfillJob :one
SELECT agent_memory_lineage_backfill_jobs.*, NOW(3) AS database_now
FROM agent_memory_lineage_backfill_jobs
WHERE job_name = ?
FOR UPDATE;

-- name: GetAgentMemoryLineageBackfillJob :one
SELECT * FROM agent_memory_lineage_backfill_jobs WHERE job_name = ?;

-- name: ClaimAgentMemoryLineageBackfillJob :exec
UPDATE agent_memory_lineage_backfill_jobs
SET status = 'running', owner_id = sqlc.arg(owner_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)),
    attempt_count = attempt_count + 1, last_error = '', completed_at = NULL
WHERE job_name = sqlc.arg(job_name);

-- name: AdvanceAgentMemoryLineageBackfillJob :execresult
UPDATE agent_memory_lineage_backfill_jobs
SET last_processed_id = sqlc.arg(last_processed_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)), last_error = ''
WHERE job_name = sqlc.arg(job_name) AND owner_id = sqlc.arg(owner_id) AND status = 'running'
  AND lease_expires_at >= NOW(3) AND last_processed_id <= sqlc.arg(last_processed_id)
  AND source_high_watermark_id >= sqlc.arg(last_processed_id);

-- name: FailAgentMemoryLineageBackfillJob :execresult
UPDATE agent_memory_lineage_backfill_jobs
SET status = 'failed', owner_id = '', lease_expires_at = NULL, last_error = sqlc.arg(last_error)
WHERE job_name = sqlc.arg(job_name) AND owner_id = sqlc.arg(owner_id) AND status = 'running';

-- name: CompleteAgentMemoryLineageBackfillJob :execresult
UPDATE agent_memory_lineage_backfill_jobs
SET status = 'completed', owner_id = '', lease_expires_at = NULL, last_error = '', completed_at = NOW(3)
WHERE job_name = sqlc.arg(job_name) AND owner_id = sqlc.arg(owner_id) AND status = 'running'
  AND last_processed_id = source_high_watermark_id;
