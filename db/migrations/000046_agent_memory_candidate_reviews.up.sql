CREATE TABLE agent_memory_candidate_reviews (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    review_uuid VARCHAR(72) NOT NULL,
    candidate_uuid VARCHAR(72) NOT NULL,
    candidate_sha256 CHAR(64) NOT NULL,
    reviewer_uuid VARCHAR(64) NOT NULL,
    decision VARCHAR(16) NOT NULL,
    reason VARCHAR(1000) NOT NULL,
    review_sha256 CHAR(64) NOT NULL,
    reviewed_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_memory_candidate_review_uuid (review_uuid),
    UNIQUE KEY uk_agent_memory_candidate_review_candidate (candidate_uuid),
    CONSTRAINT fk_agent_memory_candidate_review_candidate FOREIGN KEY (candidate_uuid) REFERENCES agent_memory_candidates(candidate_uuid) ON DELETE RESTRICT,
    CONSTRAINT chk_agent_memory_candidate_review_decision CHECK (decision IN ('accepted', 'rejected')),
    CONSTRAINT chk_agent_memory_candidate_review_hash CHECK (candidate_sha256 REGEXP '^[a-f0-9]{64}$' AND review_sha256 REGEXP '^[a-f0-9]{64}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
