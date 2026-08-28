ALTER TABLE agent_tool_invocations
    DROP CHECK chk_agent_tool_invocation_command,
    DROP COLUMN arguments_json,
    DROP COLUMN server_id,
    DROP COLUMN profile_id;
