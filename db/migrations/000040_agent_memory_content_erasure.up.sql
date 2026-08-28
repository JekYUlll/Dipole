ALTER TABLE agent_memories
    ADD COLUMN content_erased_at DATETIME(3) NULL AFTER correction_reason,
    ADD COLUMN content_erased_by_uuid VARCHAR(64) NOT NULL DEFAULT '' AFTER content_erased_at,
    ADD COLUMN content_erasure_reason_code VARCHAR(32) NOT NULL DEFAULT '' AFTER content_erased_by_uuid,
    ADD CONSTRAINT chk_agent_memory_content_erasure CHECK (
        (content_erased_at IS NULL AND content_erased_by_uuid = '' AND content_erasure_reason_code = '') OR
        (content_erased_at IS NOT NULL
            AND content_erased_by_uuid <> ''
            AND content_erasure_reason_code IN ('owner_request', 'retention_expired', 'policy_violation')
            AND status = 'revoked'
            AND content = '[erased]'
            AND compact_content IS NULL
            AND source_uri IS NULL
            AND resource_type = 'erased'
            AND resource_id = '[erased]'
            AND ((memory_version = 1 AND source_type = 'erased' AND source_id = '[erased]' AND source_sequence IS NULL) OR memory_version > 1)
            AND revoke_reason = 'privacy erasure'
            AND ((memory_version = 1 AND correction_reason = '') OR (memory_version > 1 AND correction_reason = 'privacy erasure')))
    );
