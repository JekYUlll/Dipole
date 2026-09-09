ALTER TABLE agent_mcp_tool_rounds
    DROP CONSTRAINT chk_agent_mcp_tool_round_number,
    ADD CONSTRAINT chk_agent_mcp_tool_round_number CHECK (round_number < 8);
