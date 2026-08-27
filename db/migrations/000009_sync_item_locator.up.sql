ALTER TABLE user_sync_inbox
    ADD COLUMN message_seq BIGINT UNSIGNED NULL AFTER conversation_key;

UPDATE user_sync_inbox AS inbox
JOIN messages AS message ON message.uuid = inbox.message_uuid
SET inbox.message_seq = message.seq;

ALTER TABLE user_sync_inbox
    MODIFY COLUMN message_seq BIGINT UNSIGNED NOT NULL,
    ADD UNIQUE KEY idx_sync_inbox_user_conversation_seq (user_uuid, conversation_key, message_seq);
