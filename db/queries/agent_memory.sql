-- name: InsertAgentMemory :exec
INSERT INTO agent_memories (
    memory_uuid, tenant_id, principal_uuid, agent_uuid, memory_type, status,
    resource_type, resource_id, content, compact_content, priority,
    source_type, source_id, source_uri, source_sequence,
    valid_from, expires_at, revoked_at,
    memory_root_uuid, memory_version, supersedes_memory_uuid, corrected_by_uuid, correction_reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAgentContextMemories :many
SELECT *
FROM agent_memories
WHERE tenant_id = ?
  AND principal_uuid = ?
  AND agent_uuid = ?
  AND resource_type = ?
  AND resource_id = ?
  AND created_at <= ?
  AND status = 'active'
  AND revoked_at IS NULL
  AND valid_from <= ?
  AND (expires_at IS NULL OR expires_at > ?)
ORDER BY priority DESC, memory_uuid ASC
LIMIT ?;

-- name: RevokeAgentMemory :execrows
UPDATE agent_memories
SET status = 'revoked',
    revoked_at = ?,
    revoked_by_uuid = principal_uuid,
    revoke_reason = 'legacy internal revocation'
WHERE memory_uuid = ? AND status = 'active' AND revoked_at IS NULL;

-- name: ListOwnedAgentMemories :many
SELECT *
FROM agent_memories
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND (
    created_at < sqlc.arg(after_created_at)
    OR (created_at = sqlc.arg(after_created_at) AND memory_uuid > sqlc.arg(after_memory_uuid))
  )
ORDER BY created_at DESC, memory_uuid ASC
LIMIT ?;

-- name: GetOwnedAgentMemory :one
SELECT *
FROM agent_memories
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND memory_uuid = sqlc.arg(memory_uuid)
LIMIT 1;

-- name: GetOwnedAgentMemoryForUpdate :one
SELECT *
FROM agent_memories
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND memory_uuid = sqlc.arg(memory_uuid)
LIMIT 1
FOR UPDATE;

-- name: GetAgentMemoryBySupersedes :one
SELECT *
FROM agent_memories
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND supersedes_memory_uuid = sqlc.arg(supersedes_memory_uuid)
LIMIT 1;

-- name: ListOwnedAgentMemoryRootForUpdate :many
SELECT *
FROM agent_memories
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND memory_root_uuid = sqlc.arg(memory_root_uuid)
ORDER BY memory_version ASC
FOR UPDATE;

-- name: RevokeOwnedAgentMemory :execrows
UPDATE agent_memories
SET status = 'revoked',
    revoked_at = sqlc.arg(revoked_at),
    revoked_by_uuid = sqlc.arg(revoked_by_uuid),
    revoke_reason = sqlc.arg(revoke_reason)
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND memory_uuid = sqlc.arg(memory_uuid)
  AND status = 'active'
  AND revoked_at IS NULL;

-- name: SupersedeOwnedAgentMemory :execrows
UPDATE agent_memories
SET status = 'revoked',
    revoked_at = sqlc.arg(revoked_at),
    revoked_by_uuid = sqlc.arg(revoked_by_uuid),
    revoke_reason = sqlc.arg(revoke_reason)
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND memory_uuid = sqlc.arg(memory_uuid)
  AND memory_version = sqlc.arg(memory_version)
  AND status = 'active'
  AND revoked_at IS NULL;

-- name: EraseOwnedAgentMemoryRoot :execrows
UPDATE agent_memories
SET status = 'revoked',
    revoked_at = COALESCE(revoked_at, sqlc.arg(content_erased_at)),
    revoked_by_uuid = sqlc.arg(content_erased_by_uuid),
    revoke_reason = 'privacy erasure',
    content = '[erased]',
    compact_content = NULL,
    source_uri = NULL,
    resource_type = 'erased',
    resource_id = '[erased]',
    source_type = CASE WHEN memory_version = 1 THEN 'erased' ELSE source_type END,
    source_id = CASE WHEN memory_version = 1 THEN '[erased]' ELSE source_id END,
    source_sequence = CASE WHEN memory_version = 1 THEN NULL ELSE source_sequence END,
    correction_reason = CASE WHEN memory_version = 1 THEN '' ELSE 'privacy erasure' END,
    content_erased_at = sqlc.arg(content_erased_at),
    content_erased_by_uuid = sqlc.arg(content_erased_by_uuid),
    content_erasure_reason_code = sqlc.arg(content_erasure_reason_code)
WHERE tenant_id = sqlc.arg(tenant_id)
  AND principal_uuid = sqlc.arg(principal_uuid)
  AND memory_root_uuid = sqlc.arg(memory_root_uuid)
  AND content_erased_at IS NULL;
