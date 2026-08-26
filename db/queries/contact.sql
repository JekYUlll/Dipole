-- name: CreateFriendship :execresult
INSERT INTO contacts (
    user_uuid,
    friend_uuid,
    remark,
    status,
    created_at,
    updated_at
) VALUES
    (sqlc.arg(user_one_uuid), sqlc.arg(user_two_uuid), '', sqlc.arg(status), NOW(3), NOW(3)),
    (sqlc.arg(user_two_uuid), sqlc.arg(user_one_uuid), '', sqlc.arg(status), NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE id = id;

-- name: DeleteFriendship :execresult
DELETE FROM contacts
WHERE (user_uuid = sqlc.arg(user_one_uuid) AND friend_uuid = sqlc.arg(user_two_uuid))
   OR (user_uuid = sqlc.arg(user_two_uuid) AND friend_uuid = sqlc.arg(user_one_uuid));

-- name: ListContactsByUser :many
SELECT id, user_uuid, friend_uuid, remark, status, created_at, updated_at
FROM contacts
WHERE user_uuid = ?
ORDER BY created_at DESC;

-- name: GetContact :one
SELECT id, user_uuid, friend_uuid, remark, status, created_at, updated_at
FROM contacts
WHERE user_uuid = ? AND friend_uuid = ?
LIMIT 1;

-- name: UpdateContact :execresult
UPDATE contacts
SET remark = ?,
    status = ?,
    updated_at = NOW(3)
WHERE user_uuid = ? AND friend_uuid = ?;

-- name: CreateContactApplication :execresult
INSERT INTO contact_applications (
    applicant_uuid,
    target_uuid,
    message,
    status,
    expires_at,
    handled_at,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: GetContactApplicationByPair :one
SELECT id, applicant_uuid, target_uuid, message, status, expires_at,
       handled_at, created_at, updated_at
FROM contact_applications
WHERE applicant_uuid = ? AND target_uuid = ?
LIMIT 1;

-- name: GetContactApplicationByID :one
SELECT id, applicant_uuid, target_uuid, message, status, expires_at,
       handled_at, created_at, updated_at
FROM contact_applications
WHERE id = ?
LIMIT 1;

-- name: UpdateContactApplication :execresult
UPDATE contact_applications
SET message = ?,
    status = ?,
    expires_at = ?,
    handled_at = ?,
    updated_at = NOW(3)
WHERE id = ?;

-- name: ListIncomingContactApplications :many
SELECT id, applicant_uuid, target_uuid, message, status, expires_at,
       handled_at, created_at, updated_at
FROM contact_applications
WHERE target_uuid = ?
ORDER BY created_at DESC;

-- name: ListOutgoingContactApplications :many
SELECT id, applicant_uuid, target_uuid, message, status, expires_at,
       handled_at, created_at, updated_at
FROM contact_applications
WHERE applicant_uuid = ?
ORDER BY created_at DESC;
