-- name: CountPublishedSearchOutboxThrough :one
SELECT COUNT(*) FROM outbox_events
WHERE id <= sqlc.arg(through_id)
  AND aggregate_type = 'message'
  AND status = 'published'
  AND event_type IN (
    'message.direct.created', 'message.direct.edited', 'message.direct.recalled', 'message.direct.deleted',
    'message.group.created', 'message.group.edited', 'message.group.recalled', 'message.group.deleted'
  );

-- name: CountNonPublishedSearchOutboxThrough :one
SELECT COUNT(*) FROM outbox_events
WHERE id <= sqlc.arg(through_id)
  AND aggregate_type = 'message'
  AND status <> 'published'
  AND event_type IN (
    'message.direct.created', 'message.direct.edited', 'message.direct.recalled', 'message.direct.deleted',
    'message.group.created', 'message.group.edited', 'message.group.recalled', 'message.group.deleted'
  );

-- name: DeletePublishedSearchOutboxBatch :execresult
DELETE FROM outbox_events
WHERE id <= sqlc.arg(through_id)
  AND aggregate_type = 'message'
  AND status = 'published'
  AND event_type IN (
    'message.direct.created', 'message.direct.edited', 'message.direct.recalled', 'message.direct.deleted',
    'message.group.created', 'message.group.edited', 'message.group.recalled', 'message.group.deleted'
  )
ORDER BY id ASC
LIMIT ?;
