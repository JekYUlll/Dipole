-- name: CreateGroup :execresult
INSERT INTO `groups` (
    uuid,
    name,
    notice,
    avatar,
    avatar_file_uuid,
    owner_uuid,
    member_count,
    status,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateGroupMember :execresult
INSERT INTO group_members (
    group_uuid,
    user_uuid,
    role,
    joined_at,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?)
;

-- name: AddGroupMember :execresult
INSERT INTO group_members (
    group_uuid,
    user_uuid,
    role,
    joined_at,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE id = id;

-- name: GetGroupByUUID :one
SELECT id, uuid, name, notice, avatar, avatar_file_uuid, owner_uuid,
       member_count, status, created_at, updated_at
FROM `groups`
WHERE uuid = ?
LIMIT 1;

-- name: GetGroupMember :one
SELECT id, group_uuid, user_uuid, role, joined_at, created_at, updated_at
FROM group_members
WHERE group_uuid = ? AND user_uuid = ?
LIMIT 1;

-- name: ListGroupMembers :many
SELECT id, group_uuid, user_uuid, role, joined_at, created_at, updated_at
FROM group_members
WHERE group_uuid = ?
ORDER BY role ASC, joined_at ASC;

-- name: UpdateGroup :execresult
UPDATE `groups`
SET name = ?,
    notice = ?,
    avatar = ?,
    avatar_file_uuid = ?,
    owner_uuid = ?,
    member_count = ?,
    status = ?,
    updated_at = NOW(3)
WHERE uuid = ?;

-- name: AdjustGroupMemberCount :execresult
UPDATE `groups`
SET member_count = member_count + ?,
    updated_at = NOW(3)
WHERE uuid = ?;

-- name: DeleteGroupMembers :execresult
DELETE FROM group_members
WHERE group_uuid = ?
  AND user_uuid IN (sqlc.slice('user_uuids'));

-- name: DeleteGroupMember :execresult
DELETE FROM group_members
WHERE group_uuid = ? AND user_uuid = ?;
