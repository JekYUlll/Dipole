ALTER TABLE message_metadata
    ADD COLUMN legacy_message_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER message_uuid;

UPDATE message_metadata AS metadata
JOIN messages ON messages.uuid = metadata.message_uuid
SET metadata.legacy_message_id = messages.id;
