DROP TABLE IF EXISTS device_sync_checkpoints;

ALTER TABLE conversations
    DROP COLUMN read_seq,
    DROP COLUMN last_message_seq;
