-- name: GetSearchBackfillHighWatermark :one
SELECT id FROM outbox_events
WHERE aggregate_type = 'message'
  AND event_type IN (
    'message.direct.created', 'message.direct.edited', 'message.direct.recalled', 'message.direct.deleted',
    'message.group.created', 'message.group.edited', 'message.group.recalled', 'message.group.deleted'
  )
ORDER BY id DESC LIMIT 1;

-- name: ListLatestSearchMutationsForBackfill :many
SELECT candidate.id, candidate.event_type, candidate.message_key, candidate.value
FROM outbox_events candidate
WHERE candidate.id > sqlc.arg(after_id)
  AND candidate.id <= sqlc.arg(through_id)
  AND candidate.aggregate_type = 'message'
  AND candidate.event_type IN (
    'message.direct.created', 'message.direct.edited', 'message.direct.recalled', 'message.direct.deleted',
    'message.group.created', 'message.group.edited', 'message.group.recalled', 'message.group.deleted'
  )
  AND NOT EXISTS (
    SELECT 1 FROM outbox_events newer
    WHERE newer.message_key = candidate.message_key
      AND newer.id > candidate.id
      AND newer.id <= sqlc.arg(through_id)
      AND newer.aggregate_type = 'message'
      AND newer.event_type IN (
        'message.direct.created', 'message.direct.edited', 'message.direct.recalled', 'message.direct.deleted',
        'message.group.created', 'message.group.edited', 'message.group.recalled', 'message.group.deleted'
      )
  )
ORDER BY candidate.id ASC
LIMIT ?;

-- name: EnsureSearchBackfillJob :exec
INSERT INTO search_backfill_jobs (
  job_name, target_index, source_kind, source_snapshot_id, source_sha256,
  source_high_watermark_id, last_error
)
VALUES (?, ?, ?, ?, ?, ?, '')
ON DUPLICATE KEY UPDATE job_name = VALUES(job_name);

-- name: LockSearchBackfillJob :one
SELECT search_backfill_jobs.*, NOW(3) AS database_now FROM search_backfill_jobs
WHERE job_name = ?
FOR UPDATE;

-- name: GetSearchBackfillJob :one
SELECT * FROM search_backfill_jobs WHERE job_name = ?;

-- name: ClaimSearchBackfillJob :exec
UPDATE search_backfill_jobs
SET status = 'running',
    owner_id = sqlc.arg(owner_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)),
    attempt_count = attempt_count + 1,
    last_error = '',
    completed_at = NULL
WHERE job_name = sqlc.arg(job_name);

-- name: AdvanceSearchBackfillJob :execresult
UPDATE search_backfill_jobs
SET last_processed_id = sqlc.arg(last_processed_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)),
    last_error = ''
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running'
  AND lease_expires_at >= NOW(3)
  AND last_processed_id <= sqlc.arg(last_processed_id)
  AND source_high_watermark_id >= sqlc.arg(last_processed_id);

-- name: FailSearchBackfillJob :execresult
UPDATE search_backfill_jobs
SET status = 'failed',
    owner_id = '',
    lease_expires_at = NULL,
    last_error = sqlc.arg(last_error)
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running';

-- name: CompleteSearchBackfillJob :execresult
UPDATE search_backfill_jobs
SET status = 'completed',
    owner_id = '',
    lease_expires_at = NULL,
    last_error = '',
    completed_at = NOW(3)
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running'
  AND last_processed_id = source_high_watermark_id;
