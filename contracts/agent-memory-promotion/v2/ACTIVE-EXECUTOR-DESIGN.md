# Agent Memory Promotion Active Executor v1

## 目标与边界

本契约定义 reviewed Memory receipt v2 从 Temporal Activity 提交到 Core 的最小权威路径。它仅为已审核 candidate 写入一条已有类型策略允许的持久 Memory；不会启用自动 candidate 生成、跨 Task 召回、正文读取、Gateway owner 控制面或对外 HTTP 写入口。

当前 `PromoteMemoryCandidate` 是 Gateway owner 控制 RPC。`dipole-agent` 不得调用该方法，也不得传入或伪造 `principal_user_id`。receipt 的 SHA-256 只用于完整性和审计关联，不能作为授权凭据。

## 拟新增内部 RPC

`CommitReviewedMemoryPromotion` 只接受 mTLS 和 service metadata 已认证的 `dipole-agent` 调用方，且只在显式 active Runtime 配置中注册。请求字段保持低敏：

```text
receipt_id
receipt_sha256
schema_version
status
task_id
run_id
candidate_id
candidate_sha256
review_id
policy_version
target_memory_type
created_at
expires_at
```

请求不携带 owner、tenant、agent、candidate summary、review reason、正文或凭据。`schema_version=receipt.v2`、`status=prepared`、时间窗和 policy 用于重算 canonical receipt body；Core 从持久化的 Task/Run/Invocation 恢复身份和 scope。

## Core 提交算法

1. 校验 caller 为 `dipole-agent`，请求字段格式严格且 `target_memory_type` 为持久类型；`working` 直接拒绝。
2. 锁定并解析 `task_id` 与 `run_id`，要求 Run 属于该 Task、仍为 `running`，且 admission 的 `runtime_id=dipole-agent`、`mode=active`、candidate version 与当前有效 Runtime promotion grant 一致。
3. 从已解析 Invocation 获得 tenant、principal owner 与 agent；不得信任客户端声称的身份。
4. 重新计算 receipt v2 canonical body/hash，并要求其 Task/Run、owner、tenant、agent、candidate/review、目标类型和未过期窗口全部匹配。Core 将 receipt hash 作为审计值记录，授权继续由第 2 步的持久状态决定。
5. 使用已恢复的 owner 身份调用现有 candidate/review promotion 事务。该事务必须再次锁定 candidate 和 accepted review、复核 hash、类型、owner、年龄和既有 promotion。
6. 返回稳定 Memory 标识、类型、provenance 和 receipt hash。对相同 receipt 的 Temporal Activity 重试收敛到同一条 Memory；任何字段漂移、过期、终态 Run、失效 grant 或候选状态变化均拒绝，不产生新 Memory。

## 生命周期与回滚

Temporal Workflow 先持久化 `prepared` receipt，再调用提交 Activity。Activity 由 Temporal 重试；Core promotion 的既有唯一性保证处理 crash-after-commit。成功后 Workflow 只记录低敏 `committed` 结果，失败保留 `prepared` receipt 和固定错误分类，供人工 owner/Gateway 路径重新审核。

默认配置不注册该 Activity、不放行该 RPC，也不创建 Network/Provider/Memory 写副作用。回滚删除 active Runtime overlay 或关闭专用开关即可恢复 receipt-only 行为；已提交的 owner-reviewed Memory 保持可审计和可撤销。

## 必要验证

- Core/gRPC：拒绝 Gateway、未认证 caller、伪造 owner、缺失或错误 Task/Run、非 active admission、过期或撤销 Runtime grant。
- Core/application：拒绝 receipt hash、candidate/review/type/tenant/agent drift，拒绝 `working`，并确认重复提交只返回同一 Memory。
- Temporal：Worker replacement、Activity retry 和 crash-after-Core-commit 不重复写入；terminal Task/Run 不能恢复提交。
- Runtime：active profile 可构造专用 Activity；foundation、shadow 与 metadata profile 保持零调用。
- 集成：先在隔离 MySQL/Temporal 环境完成授权、撤销和回滚演练，再考虑共享环境 Evidence；该契约本身不构成 active Runtime 上线证据。
