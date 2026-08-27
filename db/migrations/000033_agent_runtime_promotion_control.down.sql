DROP TABLE IF EXISTS agent_runtime_promotion_revocations;
DROP TABLE IF EXISTS agent_runtime_promotion_reviews;
DROP TABLE IF EXISTS agent_runtime_promotion_proposals;
DROP TABLE IF EXISTS agent_runtime_promotion_operator_grants;

ALTER TABLE agent_runtime_promotion_grants
    ADD CONSTRAINT chk_agent_runtime_promotion_revocation
        CHECK (revoked_at IS NULL OR revoked_at >= valid_from);
