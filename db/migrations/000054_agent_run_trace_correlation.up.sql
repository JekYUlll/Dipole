ALTER TABLE agent_runs
    ADD COLUMN trace_id VARCHAR(128) NULL AFTER candidate_version,
    ADD KEY idx_agent_runs_trace_id (trace_id);
