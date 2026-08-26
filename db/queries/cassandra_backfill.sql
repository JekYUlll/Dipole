-- name: GetCassandraBackfillHighWatermark :one
SELECT id FROM messages ORDER BY id DESC LIMIT 1;

-- name: ListMessagesForCassandraBackfill :many
SELECT * FROM messages
WHERE id > sqlc.arg(after_id)
  AND id <= sqlc.arg(through_id)
ORDER BY id ASC
LIMIT ?;

-- name: EnsureCassandraBackfillJob :exec
INSERT INTO cassandra_backfill_jobs (job_name, source_high_watermark_id, last_error)
VALUES (?, ?, '')
ON DUPLICATE KEY UPDATE job_name = VALUES(job_name);

-- name: LockCassandraBackfillJob :one
SELECT cassandra_backfill_jobs.*, NOW(3) AS database_now FROM cassandra_backfill_jobs
WHERE job_name = ?
FOR UPDATE;

-- name: ClaimCassandraBackfillJob :exec
UPDATE cassandra_backfill_jobs
SET status = 'running',
    owner_id = sqlc.arg(owner_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)),
    attempt_count = attempt_count + 1,
    last_error = '',
    completed_at = NULL
WHERE job_name = sqlc.arg(job_name);

-- name: AdvanceCassandraBackfillJob :execresult
UPDATE cassandra_backfill_jobs
SET last_processed_id = sqlc.arg(last_processed_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)),
    last_error = ''
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running'
  AND lease_expires_at >= NOW(3)
  AND last_processed_id <= sqlc.arg(last_processed_id)
  AND source_high_watermark_id >= sqlc.arg(last_processed_id);

-- name: FailCassandraBackfillJob :execresult
UPDATE cassandra_backfill_jobs
SET status = 'failed',
    owner_id = '',
    lease_expires_at = NULL,
    last_error = sqlc.arg(last_error)
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running';

-- name: CompleteCassandraBackfillJob :execresult
UPDATE cassandra_backfill_jobs
SET status = 'completed',
    owner_id = '',
    lease_expires_at = NULL,
    last_error = '',
    completed_at = NOW(3)
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running'
  AND last_processed_id = source_high_watermark_id;
