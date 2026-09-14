-- A consumed approval authorizes one invocation, including retries of that invocation.
ALTER TABLE agent_tool_invocations
    DROP INDEX idx_agent_tool_invocation_approval,
    ADD UNIQUE KEY idx_agent_tool_invocation_approval (approval_uuid);
