ALTER TABLE agent_shadow_steps
    DROP COLUMN lease_expires_at,
    DROP COLUMN claim_token;

DROP TABLE IF EXISTS agent_runs;
