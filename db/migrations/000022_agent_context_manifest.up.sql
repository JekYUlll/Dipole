ALTER TABLE agent_shadow_plans
    ADD COLUMN context_compiler_version VARCHAR(16) NULL AFTER model_output_tokens,
    ADD COLUMN context_estimated_tokens INT UNSIGNED NULL AFTER context_compiler_version,
    ADD COLUMN context_manifest_json JSON NULL AFTER context_estimated_tokens,
    ADD CONSTRAINT chk_agent_shadow_plans_context CHECK (
        (context_compiler_version IS NULL AND context_estimated_tokens IS NULL AND context_manifest_json IS NULL) OR
        (context_compiler_version IS NOT NULL AND context_estimated_tokens > 0 AND context_manifest_json IS NOT NULL)
    );
