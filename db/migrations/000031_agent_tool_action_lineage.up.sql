ALTER TABLE agent_tool_invocations
    ADD COLUMN approval_uuid VARCHAR(64) NULL AFTER trace_id,
    ADD COLUMN action_resource_type VARCHAR(32) NULL AFTER error_code,
    ADD COLUMN action_resource_uuid VARCHAR(64) NULL AFTER action_resource_type,
    ADD COLUMN action_command_kind VARCHAR(32) NULL AFTER action_resource_uuid,
    ADD COLUMN action_command_id VARCHAR(128) NULL AFTER action_command_kind,
    ADD KEY idx_agent_tool_invocation_approval (approval_uuid),
    ADD KEY idx_agent_tool_invocation_action_resource (action_resource_type, action_resource_uuid),
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
