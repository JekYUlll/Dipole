-- name: GetAgentMemoryDerivedImpact :one
SELECT
    selected.memory_root_uuid,
    (SELECT COUNT(*) FROM agent_memories AS versions
        WHERE versions.tenant_id = selected.tenant_id
          AND versions.principal_uuid = selected.principal_uuid
          AND versions.memory_root_uuid = selected.memory_root_uuid) AS lineage_versions,
    (SELECT COUNT(DISTINCT lineage.task_uuid) FROM agent_memory_task_lineage AS lineage
        JOIN agent_memories AS referenced ON referenced.memory_uuid = lineage.memory_uuid
        WHERE referenced.tenant_id = selected.tenant_id
          AND referenced.principal_uuid = selected.principal_uuid
          AND referenced.memory_root_uuid = selected.memory_root_uuid) AS direct_task_references,
    (SELECT COUNT(*) FROM agent_shadow_plans AS legacy_plans
        WHERE legacy_plans.context_manifest_json IS NOT NULL
          AND EXISTS (
            SELECT 1 FROM agent_memories AS legacy_versions
            WHERE legacy_versions.tenant_id = selected.tenant_id
              AND legacy_versions.principal_uuid = selected.principal_uuid
              AND legacy_versions.memory_root_uuid = selected.memory_root_uuid
              AND JSON_SEARCH(
                legacy_plans.context_manifest_json,
                'one',
                CONCAT('memory:', REPLACE(legacy_versions.memory_uuid, '_', '!_')),
                '!',
                '$.selected[*].id'
              ) IS NOT NULL)
          AND NOT EXISTS (
            SELECT 1 FROM agent_memory_task_lineage AS indexed
            JOIN agent_memories AS indexed_memory ON indexed_memory.memory_uuid = indexed.memory_uuid
            WHERE indexed.task_uuid = legacy_plans.task_uuid
              AND indexed_memory.memory_root_uuid = selected.memory_root_uuid)) AS unindexed_context_plans,
    (SELECT COUNT(DISTINCT orphan_runs.task_uuid)
        FROM agent_model_runs AS orphan_runs
        JOIN agent_model_calls AS orphan_calls ON orphan_calls.run_uuid = orphan_runs.run_uuid
        JOIN agent_tasks AS orphan_tasks ON orphan_tasks.task_uuid = orphan_runs.task_uuid
        LEFT JOIN agent_shadow_plans AS orphan_plans ON orphan_plans.task_uuid = orphan_runs.task_uuid
        WHERE orphan_calls.status = 'completed'
          AND orphan_tasks.tenant_id = selected.tenant_id
          AND orphan_tasks.principal_uuid = selected.principal_uuid
          AND orphan_plans.task_uuid IS NULL) AS unattributed_model_tasks,
    (SELECT COUNT(*) FROM agent_model_calls AS calls
        JOIN agent_model_runs AS runs ON runs.run_uuid = calls.run_uuid
        WHERE runs.task_uuid IN (
            SELECT lineage.task_uuid FROM agent_memory_task_lineage AS lineage
            JOIN agent_memories AS referenced ON referenced.memory_uuid = lineage.memory_uuid
            WHERE referenced.tenant_id = selected.tenant_id AND referenced.principal_uuid = selected.principal_uuid
              AND referenced.memory_root_uuid = selected.memory_root_uuid)) AS model_calls,
    (SELECT COUNT(*) FROM agent_shadow_plans AS plans WHERE plans.task_uuid IN (
        SELECT lineage.task_uuid FROM agent_memory_task_lineage AS lineage JOIN agent_memories AS referenced ON referenced.memory_uuid = lineage.memory_uuid
        WHERE referenced.tenant_id = selected.tenant_id AND referenced.principal_uuid = selected.principal_uuid
          AND referenced.memory_root_uuid = selected.memory_root_uuid)) AS shadow_plans,
    (SELECT COUNT(*) FROM agent_shadow_steps AS steps WHERE steps.task_uuid IN (
        SELECT lineage.task_uuid FROM agent_memory_task_lineage AS lineage JOIN agent_memories AS referenced ON referenced.memory_uuid = lineage.memory_uuid
        WHERE referenced.tenant_id = selected.tenant_id AND referenced.principal_uuid = selected.principal_uuid
          AND referenced.memory_root_uuid = selected.memory_root_uuid)) AS shadow_steps,
    (SELECT COUNT(*) FROM agent_artifacts AS artifacts WHERE artifacts.task_uuid IN (
        SELECT lineage.task_uuid FROM agent_memory_task_lineage AS lineage JOIN agent_memories AS referenced ON referenced.memory_uuid = lineage.memory_uuid
        WHERE referenced.tenant_id = selected.tenant_id AND referenced.principal_uuid = selected.principal_uuid
          AND referenced.memory_root_uuid = selected.memory_root_uuid)) AS artifacts,
    (SELECT COUNT(*) FROM agent_tool_invocations AS tools WHERE tools.task_uuid IN (
        SELECT lineage.task_uuid FROM agent_memory_task_lineage AS lineage JOIN agent_memories AS referenced ON referenced.memory_uuid = lineage.memory_uuid
        WHERE referenced.tenant_id = selected.tenant_id AND referenced.principal_uuid = selected.principal_uuid
          AND referenced.memory_root_uuid = selected.memory_root_uuid)) AS tool_invocations,
    (SELECT COUNT(*) FROM agent_tool_invocations AS actions
        WHERE actions.action_resource_type = 'message' AND actions.action_resource_uuid IS NOT NULL
          AND actions.task_uuid IN (
            SELECT lineage.task_uuid FROM agent_memory_task_lineage AS lineage JOIN agent_memories AS referenced ON referenced.memory_uuid = lineage.memory_uuid
            WHERE referenced.tenant_id = selected.tenant_id AND referenced.principal_uuid = selected.principal_uuid
              AND referenced.memory_root_uuid = selected.memory_root_uuid)) AS message_actions
FROM agent_memories AS selected
WHERE selected.tenant_id = ?
  AND selected.principal_uuid = ?
  AND selected.memory_uuid = ?
LIMIT 1;
