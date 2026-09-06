-- Revert the action-reference command-kind allowlist to the 1v1-only set.
ALTER TABLE agent_tool_invocations
    DROP CHECK chk_agent_tool_invocation_action_reference;

ALTER TABLE agent_tool_invocations
    ADD CONSTRAINT chk_agent_tool_invocation_action_reference CHECK (
        (action_resource_type IS NULL AND action_resource_uuid IS NULL AND action_command_kind IS NULL AND action_command_id IS NULL)
        OR (
            status = 'completed'
            AND approval_uuid IS NOT NULL
            AND action_resource_type = 'message'
            AND action_resource_uuid IS NOT NULL
            AND action_command_kind IN ('assistant_reply', 'system_message')
            AND action_command_id IS NOT NULL
        )
    );
