-- Route B/B2: a governed group @-mention reply records its immutable action
-- reference with command kind 'group_reply'. The original constraint
-- (migration 000031) only admitted the 1v1 'assistant_reply' / 'system_message'
-- kinds, so persisting a completed group reply's evidence failed the check and
-- the task ended failed:failed even though the reply was delivered. Widen the
-- allowlist to include 'group_reply'.
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
            AND action_command_kind IN ('assistant_reply', 'system_message', 'group_reply')
            AND action_command_id IS NOT NULL
        )
    );
