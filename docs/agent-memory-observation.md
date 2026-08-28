# Agent Memory Observation/Reflection

## 目标

`ObservationWorker` 和 `ReflectionWorker` 为 Agent Memory 提供一个框架中立、默认只读的压缩边界：

```text
message event
    -> ObservationWorker
    -> observational candidate
    -> reviewed/persisted sink
    -> ReflectionWorker
    -> reflection candidate
```

本切片只生成候选，不自动写入 `agent_memories`，也不调用模型、Kafka、数据库或外部工具。后续接入时必须通过显式 sink、owner/policy 校验和独立 rollout 开关。

## Candidate Ledger

v45 的 `agent_memory_candidates` 只保存 `compactContent` 摘要、候选来源、证据 ID 列表、策略版本、规范哈希和 `pending|accepted|rejected` 状态。完整 candidate content 不进入 ledger；候选 ID 唯一键与内容哈希冲突校验共同提供精确重放语义。

Ledger 的写入不会改变 `agent_memories`。只有后续经过 reviewer、scope/TTL/correction 校验和 Temporal durable receipt 的显式投影，才可以创建长期 Memory。

v46 增加 append-only review ledger。`accepted|rejected` 决策绑定候选哈希、reviewer、有限理由、审核时间和 review hash；候选状态更新与审核记录在同一 MySQL 事务中完成。重复决策只有在完整 review hash 一致时才返回 duplicate，候选缺失、已审核或哈希漂移均回滚并 fail closed。回滚 v46 会删除审核表，保留 v45 候选记录，不会删除 Memory。

v47 增加 promotion receipt 字段。Core 在事务内锁定 accepted candidate 和 accepted owner review，创建仅由摘要组成的 observational Memory，再写入 `promoted_memory_uuid/promoted_at`；唯一 receipt 使重试返回同一 Memory。任何候选或审核绑定漂移都会回滚。回滚 v47 需要先停止 promotion 调用，再删除 promotion receipt 字段，v45/v46 审计记录继续保留。

当前已增加 `PromoteMemoryCandidate` additive gRPC 与 Gateway HTTP 入口。入口只接受候选 ID、候选 SHA-256 和 review ID，Gateway 从 JWT 会话绑定 principal，Core 再执行 accepted/status/scope/30 天证据窗口校验。返回结果要求保持候选来源与 review ID 一致；Temporal 自动晋级和 Runtime 旁路仍关闭。

Temporal preparation Activity 现使用 `agent-memory-promotion-receipt.v1` 记录可恢复晋级意图。receipt 不复制 candidate summary 或 evidence，绑定 Task/Run、owner、候选/审核哈希、策略版本和最多 15 分钟租约；它可以在 Workflow 重放时复用，不能替代 Core v47 事务，也不能单独证明 Memory 已写入。

Memory reviewed corpus v1 采用与订阅语义评测一致的双 reviewer + 独立 adjudicator 机制。Corpus 只保存候选类型、资源范围、证据数量、脱敏内容哈希和 gold label；`eval:memory-corpus-review` 仅输出 SHA-256、计数、agreement 和门禁原因。当前夹具用于验证协议，真实语料接入前仍需隐私审查、owner 批准和 retrieval 标注验收。

真实来源通过 source manifest v1 接入：loader 在读取前校验 owner UID、绝对规范路径、父目录 canonical、`O_NOFOLLOW`、regular/single-link、0600 权限、2 MiB 上限、批准窗口及 corpus/review SHA-256。manifest 只授权离线评测，不授予 Memory 写入、Runtime 切流或晋级权限。

Memory prefilter evidence v1 进一步将 embedding/small_model 的逐 case 分数、阈值、延迟和成本绑定 reviewed corpus。`eval:memory-prefilter` 只生成低敏聚合报告，供候选比较和后续灰度门禁使用；当前不调用真实模型、不消费 Kafka，也不改变自动 Memory 写入开关。

## 不变量

- Observation 以 `eventId` 幂等；同一 worker 重复收到事件不会生成第二个候选。
- Candidate ID 由租户、主体、资源、事件/窗口和证据 ID 的规范字节计算，重放不会产生漂移 ID。
- 候选固定为 `observational` 和 `untrusted` 语义，provenance 只保存消息或窗口引用，不复制凭据。
- 输入超限、凭据模式和无法识别的观察内容 fail closed；上游坏数据不会中断消费循环。
- Reflection 需要最少两个唯一观察，拒绝跨租户、跨 Agent、跨资源和重复证据。
- Reflection 窗口只能成功一次；重复窗口返回空结果，避免重复长期记忆。

## 后续接线顺序

1. 以受审阅 corpus 校准规则、小模型或 embedding 预筛选，并补充 retrieval/semantic Eval。
2. 接入持久 candidate ledger，保存 candidate hash、evidence IDs、policy version 和 reviewer decision。
3. 在 Temporal durable task 中运行 Reflection，完成取消、重试和输出 receipt 后再允许自动生成 promotion request。
4. 通过 owner scope、TTL、correction/supersession 和派生数据 retention 门禁后，按 tenant shadow 灰度。
