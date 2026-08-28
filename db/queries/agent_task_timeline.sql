-- name: InsertAgentTaskTimelineEvent :execresult
INSERT INTO agent_task_timeline_events (
    event_uuid, task_uuid, run_uuid, event_kind, status, capability_id, approval_uuid, occurred_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE event_seq = event_seq;

-- name: GetAgentTaskTimelineEvent :one
SELECT * FROM agent_task_timeline_events
WHERE task_uuid = ? AND event_seq = ?
LIMIT 1;

-- name: ListAgentTaskTimelineEvents :many
SELECT * FROM agent_task_timeline_events
WHERE task_uuid = ? AND event_seq > ?
ORDER BY event_seq ASC
LIMIT ?;
