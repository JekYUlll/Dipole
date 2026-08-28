# Agent Memory Lineage Backfill Contract v1

该契约描述历史 `agent_shadow_plans.id` 到 `agent_memory_task_lineage` 的有界回填。Manifest 固定 source high-water mark 与批大小；Receipt 只保存计数、游标和 SHA-256，不保存 tenant、principal、Task、Memory 或正文。

回填只接受历史 Context manifest 中精确的 `memory:<id>` 引用，并由数据库按 Task 的 tenant/principal 再次匹配 Memory。未知引用、representation 漂移、跨 owner 引用和游标跳跃都必须 fail closed。重复执行允许返回 duplicate 并收敛同一索引状态。

`contentRead=false`、`deletionAuthority=false` 与 `runtimeAuthority=false` 固定为 false。该契约不授权内容读取、删除、公开 API 或 Agent Runtime admission；生产执行必须额外具备注入式 checkpoint/lease target。
