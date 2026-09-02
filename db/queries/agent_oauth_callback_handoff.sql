-- name: InsertAgentOAuthCallbackHandoff :execrows
INSERT IGNORE INTO agent_oauth_callback_handoffs (
    handoff_uuid, transaction_uuid, owner_user_uuid, issuer, redirect_uri,
    authorization_code_sha256, sealed_authorization_code, runtime_key_id,
    status, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'callback_recorded', ?);

-- name: GetAgentOAuthCallbackHandoff :one
SELECT * FROM agent_oauth_callback_handoffs WHERE handoff_uuid = ? LIMIT 1;

-- name: ClaimAgentOAuthCallbackHandoff :execrows
UPDATE agent_oauth_callback_handoffs
SET status = 'exchange_claimed',
    lease_owner = ?,
    lease_expires_at = ?,
    attempts = attempts + 1
WHERE handoff_uuid = ?
  AND expires_at > ?
  AND ? < expires_at
  AND (
      status = 'callback_recorded'
      OR (status = 'exchange_claimed' AND lease_expires_at <= ?)
  );

-- name: CompleteAgentOAuthCallbackHandoff :execrows
UPDATE agent_oauth_callback_handoffs
SET status = 'exchanged',
    lease_owner = NULL,
    lease_expires_at = NULL,
    completed_at = ?
WHERE handoff_uuid = ?
  AND status = 'exchange_claimed'
  AND lease_owner = ?
  AND lease_expires_at > ?
  AND expires_at > ?;

-- name: ReleaseAgentOAuthCallbackHandoff :execrows
UPDATE agent_oauth_callback_handoffs
SET status = 'callback_recorded',
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE handoff_uuid = ?
  AND status = 'exchange_claimed'
  AND lease_owner = ?
  AND lease_expires_at > ?;
