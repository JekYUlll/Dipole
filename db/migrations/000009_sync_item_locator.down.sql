ALTER TABLE user_sync_inbox
    DROP INDEX idx_sync_inbox_user_conversation_seq,
    DROP COLUMN message_seq;
