-- name: InsertAgentTaskTimelineEvent :execresult
INSERT INTO agent_task_timeline_events (
    event_uuid, task_uuid, run_uuid, event_kind, status, capability_id, approval_uuid, occurred_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE event_seq = event_seq;

-- name: GetAgentTaskTimelineEvent :one
SELECT * FROM agent_task_timeline_events
WHERE task_uuid = ? AND event_seq = ?
LIMIT 1;

-- name: GetAgentTaskTimelineEventByUUID :one
SELECT * FROM agent_task_timeline_events
WHERE event_uuid = ?
LIMIT 1;

-- name: ListAgentTaskTimelineEvents :many
SELECT * FROM agent_task_timeline_events
WHERE task_uuid = ? AND event_seq > ?
ORDER BY event_seq ASC
LIMIT ?;

-- name: EnqueueAgentTaskTimelineRepair :execresult
INSERT INTO agent_task_timeline_repairs (
    event_uuid, task_uuid, run_uuid, event_kind, status, capability_id, approval_uuid, occurred_at,
    repair_status, retry_count, last_error, next_retry_at, locked_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', 0, ?, NOW(3), NULL)
ON DUPLICATE KEY UPDATE event_uuid = event_uuid;

-- name: SelectClaimableAgentTaskTimelineRepairs :many
SELECT event_uuid, task_uuid, run_uuid, event_kind, status, capability_id, approval_uuid, occurred_at,
       repair_status, retry_count, last_error, next_retry_at, locked_at, created_at, updated_at
FROM agent_task_timeline_repairs
WHERE ((repair_status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= ?))
    OR (repair_status = 'processing' AND locked_at IS NOT NULL AND locked_at <= ?))
ORDER BY created_at ASC, event_uuid ASC
LIMIT ?
FOR UPDATE SKIP LOCKED;

-- name: MarkAgentTaskTimelineRepairsProcessing :execresult
UPDATE agent_task_timeline_repairs
SET repair_status = 'processing', locked_at = ?, updated_at = NOW(3)
WHERE event_uuid IN (sqlc.slice('event_uuids'));

-- name: MarkAgentTaskTimelineRepairCompleted :execresult
UPDATE agent_task_timeline_repairs
SET repair_status = 'completed', locked_at = NULL, last_error = NULL, updated_at = NOW(3)
WHERE event_uuid = ?;

-- name: MarkAgentTaskTimelineRepairRetry :execresult
UPDATE agent_task_timeline_repairs
SET repair_status = 'pending', retry_count = ?, next_retry_at = ?, locked_at = NULL,
    last_error = ?, updated_at = NOW(3)
WHERE event_uuid = ?;
