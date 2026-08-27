ALTER TABLE agent_shadow_plans
    DROP CHECK chk_agent_shadow_plans_context,
    DROP COLUMN context_manifest_json,
    DROP COLUMN context_estimated_tokens,
    DROP COLUMN context_compiler_version;
