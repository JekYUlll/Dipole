ALTER TABLE agent_runs
    DROP COLUMN candidate_version;

DROP TABLE IF EXISTS agent_runtime_promotion_grants;
