-- name: GetSyncReplayHighWatermark :one
SELECT id FROM outbox_events
WHERE aggregate_type = 'message'
  AND event_type IN ('message.direct.created', 'message.group.created')
ORDER BY id DESC LIMIT 1;

-- name: ListSyncReplayEvents :many
SELECT id, event_type, message_key, value
FROM outbox_events
WHERE id > sqlc.arg(after_id)
  AND id <= sqlc.arg(through_id)
  AND aggregate_type = 'message'
  AND event_type IN ('message.direct.created', 'message.group.created')
ORDER BY id ASC
LIMIT ?;

-- name: EnsureSyncReplayJob :exec
INSERT INTO sync_replay_jobs (job_name, source_high_watermark_id, last_error)
VALUES (?, ?, '')
ON DUPLICATE KEY UPDATE job_name = VALUES(job_name);

-- name: LockSyncReplayJob :one
SELECT sync_replay_jobs.*, NOW(3) AS database_now FROM sync_replay_jobs
WHERE job_name = ?
FOR UPDATE;

-- name: GetSyncReplayJob :one
SELECT * FROM sync_replay_jobs WHERE job_name = ?;

-- name: ClaimSyncReplayJob :exec
UPDATE sync_replay_jobs
SET status = 'running',
    owner_id = sqlc.arg(owner_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)),
    attempt_count = attempt_count + 1,
    last_error = '',
    completed_at = NULL
WHERE job_name = sqlc.arg(job_name);

-- name: AdvanceSyncReplayJob :execresult
UPDATE sync_replay_jobs
SET last_processed_id = sqlc.arg(last_processed_id),
    lease_expires_at = TIMESTAMPADD(SECOND, sqlc.arg(lease_seconds), NOW(3)),
    last_error = ''
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running'
  AND lease_expires_at >= NOW(3)
  AND last_processed_id <= sqlc.arg(last_processed_id)
  AND source_high_watermark_id >= sqlc.arg(last_processed_id);

-- name: FailSyncReplayJob :execresult
UPDATE sync_replay_jobs
SET status = 'failed', owner_id = '', lease_expires_at = NULL, last_error = sqlc.arg(last_error)
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running';

-- name: CompleteSyncReplayJob :execresult
UPDATE sync_replay_jobs
SET status = 'completed', owner_id = '', lease_expires_at = NULL,
    last_error = '', completed_at = NOW(3)
WHERE job_name = sqlc.arg(job_name)
  AND owner_id = sqlc.arg(owner_id)
  AND status = 'running'
  AND last_processed_id = source_high_watermark_id;
