-- name: SelectClaimableOutboxEvents :many
SELECT id, aggregate_type, aggregate_id, event_type, topic, message_key,
       value, headers_json, status, retry_count, last_error, next_retry_at,
       locked_at, published_at, created_at, updated_at
FROM outbox_events
WHERE (
    (status = sqlc.arg(pending_status)
        AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now)))
    OR
    (status = sqlc.arg(processing_status)
        AND locked_at IS NOT NULL
        AND locked_at <= sqlc.arg(claim_before))
)
ORDER BY id ASC
LIMIT ?
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventsProcessing :execresult
UPDATE outbox_events
SET status = ?,
    locked_at = ?,
    updated_at = NOW(3)
WHERE id IN (sqlc.slice('ids'));

-- name: MarkOutboxPublished :execresult
UPDATE outbox_events
SET status = ?,
    published_at = ?,
    locked_at = NULL,
    last_error = '',
    updated_at = NOW(3)
WHERE id = ?;

-- name: MarkOutboxRetry :execresult
UPDATE outbox_events
SET status = ?,
    retry_count = ?,
    next_retry_at = ?,
    locked_at = NULL,
    last_error = ?,
    updated_at = NOW(3)
WHERE id = ?;
