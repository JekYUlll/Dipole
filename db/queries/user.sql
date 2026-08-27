-- name: CreateUser :execresult
INSERT INTO users (
    uuid,
    nickname,
    telephone,
    email,
    avatar,
    avatar_file_uuid,
    signature,
    password_hash,
    is_admin,
    user_type,
    status,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: UpsertAssistantUser :execresult
INSERT INTO users (
    uuid,
    nickname,
    telephone,
    email,
    avatar,
    avatar_file_uuid,
    signature,
    password_hash,
    is_admin,
    user_type,
    status,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
    nickname = VALUES(nickname),
    telephone = VALUES(telephone),
    email = VALUES(email),
    avatar = VALUES(avatar),
    password_hash = VALUES(password_hash),
    is_admin = VALUES(is_admin),
    user_type = VALUES(user_type),
    status = VALUES(status);

-- name: GetUserByUUID :one
SELECT id, uuid, nickname, telephone, email, avatar, avatar_file_uuid,
       signature, password_hash, is_admin, user_type, status, created_at, updated_at
FROM users
WHERE uuid = ?
LIMIT 1;

-- name: GetUserByTelephone :one
SELECT id, uuid, nickname, telephone, email, avatar, avatar_file_uuid,
       signature, password_hash, is_admin, user_type, status, created_at, updated_at
FROM users
WHERE telephone = ?
LIMIT 1;

-- name: UpdateUser :execresult
UPDATE users
SET nickname = ?,
    telephone = ?,
    email = ?,
    avatar = ?,
    avatar_file_uuid = ?,
    signature = ?,
    password_hash = ?,
    is_admin = ?,
    user_type = ?,
    status = ?,
    updated_at = NOW(3)
WHERE uuid = ?;

-- name: SearchActiveUsers :many
SELECT id, uuid, nickname, telephone, email, avatar, avatar_file_uuid,
       signature, password_hash, is_admin, user_type, status, created_at, updated_at
FROM users
WHERE status = sqlc.arg(status)
  AND (
      uuid LIKE sqlc.arg(pattern)
      OR telephone LIKE sqlc.arg(pattern)
      OR nickname LIKE sqlc.arg(pattern)
  )
  AND uuid <> sqlc.arg(exclude_uuid)
ORDER BY created_at DESC
LIMIT ?;

-- name: ListUsers :many
SELECT id, uuid, nickname, telephone, email, avatar, avatar_file_uuid,
       signature, password_hash, is_admin, user_type, status, created_at, updated_at
FROM users
WHERE (
      uuid LIKE sqlc.arg(pattern)
      OR telephone LIKE sqlc.arg(pattern)
      OR nickname LIKE sqlc.arg(pattern)
  )
ORDER BY created_at DESC
LIMIT ?;

-- name: ListUsersByStatus :many
SELECT id, uuid, nickname, telephone, email, avatar, avatar_file_uuid,
       signature, password_hash, is_admin, user_type, status, created_at, updated_at
FROM users
WHERE status = sqlc.arg(status)
  AND (
      uuid LIKE sqlc.arg(pattern)
      OR telephone LIKE sqlc.arg(pattern)
      OR nickname LIKE sqlc.arg(pattern)
  )
ORDER BY created_at DESC
LIMIT ?;

-- name: ListUsersByUUIDs :many
SELECT id, uuid, nickname, telephone, email, avatar, avatar_file_uuid,
       signature, password_hash, is_admin, user_type, status, created_at, updated_at
FROM users
WHERE uuid IN (sqlc.slice('uuids'));
