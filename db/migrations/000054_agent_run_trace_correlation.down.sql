ALTER TABLE agent_runs
    DROP KEY idx_agent_runs_trace_id,
    DROP COLUMN trace_id;
