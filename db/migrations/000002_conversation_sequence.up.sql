ALTER TABLE messages
    ADD COLUMN seq BIGINT UNSIGNED NULL AFTER conversation_key;

UPDATE messages AS target
JOIN (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY conversation_key ORDER BY id) AS conversation_seq
    FROM messages
) AS ranked ON ranked.id = target.id
SET target.seq = ranked.conversation_seq;

ALTER TABLE messages
    MODIFY COLUMN seq BIGINT UNSIGNED NOT NULL,
    ADD UNIQUE KEY idx_message_conversation_seq (conversation_key, seq);

CREATE TABLE conversation_sequences (
    conversation_key VARCHAR(64) NOT NULL,
    last_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (conversation_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO conversation_sequences (conversation_key, last_seq)
SELECT conversation_key, MAX(seq)
FROM messages
GROUP BY conversation_key;
