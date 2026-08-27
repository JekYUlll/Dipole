-- name: InsertAgentEventClaim :exec
INSERT INTO agent_event_ledger (
    event_id, task_uuid, event_type, status, claim_token, attempt_count, claimed_at, lease_expires_at
) VALUES (?, ?, ?, 'claimed', ?, 1, UTC_TIMESTAMP(), TIMESTAMPADD(MICROSECOND, ?, UTC_TIMESTAMP()));

-- name: LockAgentEventClaim :many
SELECT id, event_id, task_uuid, status, claim_token, lease_expires_at
FROM agent_event_ledger
WHERE event_id = ? OR task_uuid = ?
FOR UPDATE;

-- name: ReclaimAgentEvent :execrows
UPDATE agent_event_ledger
SET claim_token = ?,
    attempt_count = attempt_count + 1,
    claimed_at = UTC_TIMESTAMP(),
    lease_expires_at = TIMESTAMPADD(MICROSECOND, ?, UTC_TIMESTAMP()),
    last_error = NULL
WHERE id = ? AND status = 'claimed' AND claim_token = ?;

-- name: CompleteAgentEvent :execrows
UPDATE agent_event_ledger
SET status = 'completed',
    completed_at = UTC_TIMESTAMP(),
    lease_expires_at = UTC_TIMESTAMP(),
    last_error = NULL
WHERE event_id = ? AND task_uuid = ? AND claim_token = ? AND status = 'claimed'
  AND lease_expires_at >= UTC_TIMESTAMP();

-- name: ReleaseAgentEvent :execrows
UPDATE agent_event_ledger
SET lease_expires_at = UTC_TIMESTAMP(), last_error = ?
WHERE event_id = ? AND task_uuid = ? AND claim_token = ? AND status = 'claimed';

-- name: ProbeAgentEventLedger :many
SELECT event_id FROM agent_event_ledger LIMIT 1;
