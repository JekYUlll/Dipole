-- name: EnsureUserSyncState :execresult
INSERT INTO user_sync_states (user_uuid, created_at, updated_at)
VALUES (?, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE user_uuid = user_uuid;

-- name: LockUserSyncState :one
SELECT user_uuid FROM user_sync_states WHERE user_uuid = ? FOR UPDATE;

-- name: CreateUserSyncInbox :execresult
INSERT INTO user_sync_inbox (user_uuid, message_uuid, conversation_key, created_at)
VALUES (?, ?, ?, NOW(3))
ON DUPLICATE KEY UPDATE sync_seq = sync_seq;

-- name: ListUserSyncInboxAfter :many
SELECT * FROM user_sync_inbox
WHERE user_uuid = ? AND sync_seq > ?
ORDER BY sync_seq ASC
LIMIT ?;

-- name: GetLatestUserSyncSequence :one
SELECT CAST(COALESCE(MAX(sync_seq), 0) AS UNSIGNED) FROM user_sync_inbox WHERE user_uuid = ?;

-- name: GetDeviceSyncCheckpoint :one
SELECT user_uuid, device_id, sync_seq, created_at, updated_at
FROM device_sync_checkpoints
WHERE user_uuid = ? AND device_id = ?
LIMIT 1;

-- name: AdvanceDeviceSyncCheckpoint :execresult
INSERT INTO device_sync_checkpoints (user_uuid, device_id, sync_seq, created_at, updated_at)
VALUES (?, ?, ?, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
    sync_seq = GREATEST(sync_seq, VALUES(sync_seq)),
    updated_at = NOW(3);
