-- name: InsertAgentOAuthAuthorizationTransaction :execrows
INSERT IGNORE INTO agent_oauth_authorization_transactions (
    transaction_uuid, owner_user_uuid, issuer, redirect_uri, state_sha256,
    sealed_code_verifier, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetAgentOAuthAuthorizationTransaction :one
SELECT * FROM agent_oauth_authorization_transactions WHERE transaction_uuid = ? LIMIT 1;

-- name: ConsumeAgentOAuthAuthorizationTransaction :execrows
UPDATE agent_oauth_authorization_transactions
SET consumed_at = ?
WHERE transaction_uuid = ?
  AND owner_user_uuid = ?
  AND state_sha256 = ?
  AND consumed_at IS NULL
  AND expires_at > ?;
