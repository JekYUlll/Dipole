-- name: InsertAgentMemory :exec
INSERT INTO agent_memories (
    memory_uuid, tenant_id, principal_uuid, agent_uuid, memory_type, status,
    resource_type, resource_id, content, compact_content, priority,
    source_type, source_id, source_uri, source_sequence,
    valid_from, expires_at, revoked_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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
SET status = 'revoked', revoked_at = ?
WHERE memory_uuid = ? AND status = 'active' AND revoked_at IS NULL;
