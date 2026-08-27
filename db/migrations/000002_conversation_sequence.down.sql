ALTER TABLE messages
    DROP INDEX idx_message_conversation_seq,
    DROP COLUMN seq;

DROP TABLE IF EXISTS conversation_sequences;
