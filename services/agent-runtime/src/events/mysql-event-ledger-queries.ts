// Code generated from db/queries/agent_event_ledger.sql; DO NOT EDIT.

export const INSERT_AGENT_EVENT_CLAIM = "INSERT INTO agent_event_ledger (\n    event_id, task_uuid, event_type, status, claim_token, attempt_count, claimed_at, lease_expires_at\n) VALUES (?, ?, ?, 'claimed', ?, 1, UTC_TIMESTAMP(), TIMESTAMPADD(MICROSECOND, ?, UTC_TIMESTAMP()))";

export const LOCK_AGENT_EVENT_CLAIM = "SELECT id, event_id, task_uuid, status, claim_token, lease_expires_at\nFROM agent_event_ledger\nWHERE event_id = ? OR task_uuid = ?\nFOR UPDATE";

export const RECLAIM_AGENT_EVENT = "UPDATE agent_event_ledger\nSET claim_token = ?,\n    attempt_count = attempt_count + 1,\n    claimed_at = UTC_TIMESTAMP(),\n    lease_expires_at = TIMESTAMPADD(MICROSECOND, ?, UTC_TIMESTAMP()),\n    last_error = NULL\nWHERE id = ? AND status = 'claimed' AND claim_token = ?";

export const COMPLETE_AGENT_EVENT = "UPDATE agent_event_ledger\nSET status = 'completed',\n    completed_at = UTC_TIMESTAMP(),\n    lease_expires_at = UTC_TIMESTAMP(),\n    last_error = NULL\nWHERE event_id = ? AND task_uuid = ? AND claim_token = ? AND status = 'claimed'\n  AND lease_expires_at >= UTC_TIMESTAMP()";

export const RELEASE_AGENT_EVENT = "UPDATE agent_event_ledger\nSET lease_expires_at = UTC_TIMESTAMP(), last_error = ?\nWHERE event_id = ? AND task_uuid = ? AND claim_token = ? AND status = 'claimed'";

export const PROBE_AGENT_EVENT_LEDGER = "SELECT event_id FROM agent_event_ledger LIMIT 1";
