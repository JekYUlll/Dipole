-- name: GetSyncInboxHighWatermark :one
SELECT CAST(COALESCE(MAX(sync_seq), 0) AS UNSIGNED) FROM user_sync_inbox;

-- name: GetSyncCreatedOutboxBounds :one
SELECT
    CAST(COALESCE(MIN(id), 0) AS UNSIGNED) AS first_created_outbox_id,
    CAST(COALESCE(MAX(id), 0) AS UNSIGNED) AS last_created_outbox_id
FROM outbox_events
WHERE aggregate_type = 'message'
  AND event_type IN ('message.direct.created', 'message.group.created');

-- name: ListLegacySyncInboxThrough :many
SELECT inbox.sync_seq, inbox.user_uuid, inbox.message_uuid, inbox.conversation_key, inbox.message_seq
FROM user_sync_inbox AS inbox
WHERE inbox.sync_seq <= sqlc.arg(through_sync_seq)
  AND NOT EXISTS (
      SELECT 1 FROM outbox_events AS event
      WHERE event.aggregate_type = 'message'
        AND event.event_type IN ('message.direct.created', 'message.group.created')
        AND event.message_key = inbox.message_uuid
  )
ORDER BY inbox.sync_seq ASC;

-- name: ListLegacySyncInbox :many
SELECT inbox.sync_seq, inbox.user_uuid, inbox.message_uuid, inbox.conversation_key, inbox.message_seq
FROM user_sync_inbox AS inbox
WHERE NOT EXISTS (
    SELECT 1 FROM outbox_events AS event
    WHERE event.aggregate_type = 'message'
      AND event.event_type IN ('message.direct.created', 'message.group.created')
      AND event.message_key = inbox.message_uuid
)
ORDER BY inbox.sync_seq ASC;

-- name: CreateSyncInboxBaselineJob :exec
INSERT INTO sync_inbox_baseline_jobs (
    job_name, source_high_watermark_sync_seq, first_created_outbox_id,
    last_created_outbox_id, entry_count, entries_sha256
) VALUES (?, ?, ?, ?, ?, ?);

-- name: CreateSyncInboxBaselineEntry :exec
INSERT INTO sync_inbox_baseline_entries (
    job_name, sync_seq, user_uuid, message_uuid, conversation_key, message_seq
) VALUES (?, ?, ?, ?, ?, ?);

-- name: GetSyncInboxBaselineJob :one
SELECT * FROM sync_inbox_baseline_jobs WHERE job_name = ?;

-- name: ListSyncInboxBaselineEntries :many
SELECT sync_seq, user_uuid, message_uuid, conversation_key, message_seq
FROM sync_inbox_baseline_entries
WHERE job_name = ?
ORDER BY sync_seq ASC;

-- name: RestoreSyncInboxBaselineEntry :exec
INSERT INTO user_sync_inbox (
    sync_seq, user_uuid, message_uuid, conversation_key, message_seq, created_at
) VALUES (?, ?, ?, ?, ?, NOW(3));
