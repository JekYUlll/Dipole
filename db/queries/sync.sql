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

-- name: UpsertGroupSyncState :exec
INSERT INTO group_sync_states (group_uuid, latest_message_seq, latest_message_uuid, created_at, updated_at)
VALUES (?, ?, ?, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
    latest_message_uuid = IF(VALUES(latest_message_seq) >= latest_message_seq, VALUES(latest_message_uuid), latest_message_uuid),
    latest_message_seq = GREATEST(latest_message_seq, VALUES(latest_message_seq)),
    updated_at = IF(VALUES(latest_message_seq) >= latest_message_seq, NOW(3), updated_at);

-- name: GetGroupSyncState :one
SELECT group_uuid, latest_message_seq, latest_message_uuid, updated_at
FROM group_sync_states
WHERE group_uuid = ?
LIMIT 1;

-- name: ListGroupSyncCheckpoints :many
SELECT state.group_uuid, state.latest_message_seq, state.latest_message_uuid,
       COALESCE(checkpoint.pulled_message_seq, 0) AS pulled_message_seq
FROM group_sync_states AS state
LEFT JOIN device_group_sync_checkpoints AS checkpoint
  ON checkpoint.group_uuid = state.group_uuid
 AND checkpoint.user_uuid = sqlc.arg(user_uuid)
 AND checkpoint.device_id = sqlc.arg(device_id)
WHERE state.group_uuid IN (sqlc.slice('group_uuids'))
ORDER BY state.group_uuid ASC;

-- name: AdvanceDeviceGroupSyncCheckpoint :exec
INSERT INTO device_group_sync_checkpoints (
    user_uuid, device_id, group_uuid, pulled_message_seq, created_at, updated_at
) VALUES (?, ?, ?, ?, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
    pulled_message_seq = GREATEST(pulled_message_seq, VALUES(pulled_message_seq)),
    updated_at = NOW(3);
