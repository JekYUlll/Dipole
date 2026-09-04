-- name: InsertAgentOAuthTokenLifecycleFromClaim :execrows
INSERT IGNORE INTO agent_oauth_token_lifecycles (
    handoff_uuid, runtime_key_id, state, sealed_token_bundle,
    token_bundle_sha256, access_token_expires_at, scope, revocation_reason
)
SELECT agent_oauth_callback_handoffs.handoff_uuid, agent_oauth_callback_handoffs.runtime_key_id, ?, ?, ?, ?, ?, ?
FROM agent_oauth_callback_handoffs
WHERE agent_oauth_callback_handoffs.handoff_uuid = ?
  AND status = 'exchange_claimed'
  AND lease_owner = ?
  AND lease_expires_at > ?
  AND expires_at > ?;

-- name: GetAgentOAuthTokenLifecycle :one
SELECT * FROM agent_oauth_token_lifecycles WHERE handoff_uuid = ? LIMIT 1;

-- name: ExpireDueAgentOAuthTokenLifecycles :execrows
UPDATE agent_oauth_token_lifecycles
SET state = 'expired',
    sealed_token_bundle = NULL,
    token_bundle_sha256 = NULL,
    access_token_expires_at = NULL,
    scope = NULL,
    revocation_reason = NULL
WHERE state IN ('active', 'refreshed')
  AND access_token_expires_at <= ?
ORDER BY access_token_expires_at ASC
LIMIT ?;
