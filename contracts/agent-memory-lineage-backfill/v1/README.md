# Agent Memory Lineage Backfill Contract v1

该契约描述历史 `agent_shadow_plans.id` 到 `agent_memory_task_lineage` 的有界回填。Manifest 固定 source high-water mark 与批大小；Receipt 只保存计数、游标和 SHA-256，不保存 tenant、principal、Task、Memory 或正文。

回填只接受历史 Context manifest 中精确的 `memory:<id>` 引用，并由数据库按 Task 的 tenant/principal 再次匹配 Memory。未知引用、representation 漂移、跨 owner 引用和游标跳跃都必须 fail closed。重复执行允许返回 duplicate 并收敛同一索引状态。

`contentRead=false`、`deletionAuthority=false` 与 `runtimeAuthority=false` 固定为 false。该契约不授权内容读取、删除、公开 API 或 Agent Runtime admission；生产执行必须额外具备注入式 checkpoint/lease target。

## Operator flow

1. Run `dipole-agent-memory-lineage-backfill -manifest-out manifest.json -batch-size 100` to create a fixed high-water manifest. This mode only reads the source watermark.
2. A separately reviewed approver creates an approval matching the manifest hash, job name and execution operator. The approver and operator must be distinct. Its shape is defined by `approval.schema.json`; the CLI never self-approves.
3. Run with `-execute -manifest manifest.json -approval approval.json -operator <operator> -approver <approver> -receipt-out receipt.json`. The command requires the current schema, uses the manifest batch size and emits a low-sensitive receipt.

The command is intentionally unsuitable for shared execution until the deployment owner records the maintenance window and approval evidence. All contract files are capped at 64 KiB and are written with owner-only permissions.
