ALTER TABLE agent_tool_invocations
    DROP INDEX idx_agent_tool_invocation_approval,
    ADD KEY idx_agent_tool_invocation_approval (approval_uuid);
