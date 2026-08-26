-- name: CreateUploadedFile :execresult
INSERT INTO uploaded_files (
    uuid,
    uploader_uuid,
    bucket,
    object_key,
    file_name,
    file_size,
    content_type,
    url,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: GetUploadedFileByUUID :one
SELECT id, uuid, uploader_uuid, bucket, object_key, file_name, file_size,
       content_type, url, created_at, updated_at
FROM uploaded_files
WHERE uuid = ?
LIMIT 1;
