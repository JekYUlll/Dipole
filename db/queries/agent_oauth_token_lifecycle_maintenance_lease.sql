-- name: InsertAgentOAuthTokenLifecycleMaintenanceLease :execrows
INSERT IGNORE INTO agent_oauth_token_lifecycle_maintenance_leases (
    handoff_uuid, runtime_key_id, lease_owner, lease_generation, lease_expires_at
) VALUES (?, ?, ?, 1, ?);

-- name: ReclaimExpiredAgentOAuthTokenLifecycleMaintenanceLease :execrows
UPDATE agent_oauth_token_lifecycle_maintenance_leases
SET lease_owner = ?, lease_generation = lease_generation + 1, lease_expires_at = ?
WHERE handoff_uuid = ?
  AND runtime_key_id = ?
  AND lease_expires_at <= ?;

-- name: GetAgentOAuthTokenLifecycleMaintenanceLease :one
SELECT * FROM agent_oauth_token_lifecycle_maintenance_leases WHERE handoff_uuid = ? LIMIT 1;
