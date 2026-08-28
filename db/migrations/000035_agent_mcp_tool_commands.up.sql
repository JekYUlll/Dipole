ALTER TABLE agent_tool_invocations
    ADD COLUMN profile_id VARCHAR(128) NULL AFTER arguments_sha256,
    ADD COLUMN server_id VARCHAR(128) NULL AFTER profile_id,
    ADD COLUMN arguments_json JSON NULL AFTER server_id,
    ADD CONSTRAINT chk_agent_tool_invocation_command CHECK (
        (profile_id IS NULL AND server_id IS NULL AND arguments_json IS NULL)
        OR (profile_id IS NOT NULL AND server_id IS NOT NULL AND arguments_json IS NOT NULL)
    );
