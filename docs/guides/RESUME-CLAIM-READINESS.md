# 简历 Claim 验收矩阵

本文件将 Dipole IM 与 Dipole Agent 的简历表述映射到代码、可重复运行的证据和待补门禁。它服务于开发优先级，不替代两份独立的项目材料：

- [Dipole IM 项目材料](DIPOLE-IM-LEARNING-AND-INTERVIEW.md)
- [Dipole Agent 项目材料](DIPOLE-AGENT-LEARNING-AND-INTERVIEW.md)

## 1. 使用规则

只有同时具备实现、自动化验证与版本化运行报告的能力，才可填写简历中的性能数字或绝对可靠性结论。报告至少记录提交、拓扑、负载、配置摘要、样本数、指标原始数据和限制。隔离 Remote GPU 结果只能描述为开发期隔离验证。

| 状态 | 含义 |
| --- | --- |
| 可使用 | 当前实现与证据足以使用限定范围的表述。 |
| 部分完成 | 代码或隔离验证存在，但缺少 claim 所需的路径、指标或故障证据。 |
| 缺口 | 当前没有足以支持该 claim 的实现或运行证据。 |

## 2. Dipole IM

| 简历能力 | 当前状态 | 已有证据 | 写入 Claim 前的验收缺口 | 优先级 |
| --- | --- | --- | --- | --- |
| gRPC + Kafka 服务协作、Outbox 与幂等 | 部分完成 | 独立 Core/Gateway/Message/Sync/Search 入口、版本化 gRPC、Outbox；[C1 Remote GPU 基线](../../benchmarks/c1-remote-2026-08-30/README.md)在 100 在线用户下记录 `400/400` 接受、持久化、投递 | 建立消息链路故障矩阵，在 Message、Kafka consumer、Sync、Gateway/Core 重启点注入故障；每轮以 request/message/event/outbox/投影副作用的基数生成 receipt。通过后才能将“消息零丢失、零重复副作用”限定为该矩阵和负载范围内的结论。 | P0 |
| 双 Timeline、会话 Seq、Read Seq、Device Cursor | 可使用 | [消息存储与同步模型](../architecture/MESSAGE-STORAGE-AND-SYNC.md)、[Sync Service](../architecture/SYNC-SERVICE.md)与现有幂等/兼容测试 | 运行授权用户的 Sync pull 基准，归档 cursor 推进、重复事件、重连补拉和同步 P99；完成真实 Web Sync 观察窗口。 | P0 |
| Cassandra 顺序消息存储与历史 P99 | 部分完成 | Timeline schema、backfill、reconcile、shadow read、回退和 read-rollout evidence contract | 在隔离集群运行固定会话/Seq workload，证明 backfill 对账、Cassandra 候选读、MySQL fallback 与历史 P99；在该证据前不得将 Cassandra 写为默认消息事实源或填写历史 P99。 | P1 |
| Redis 路由、热点群 notify + pull、批量投影 | 部分完成 | Redis Presence/PubSub/热点策略；[AD-005 SQL 基准](../../benchmarks/ad005-conversation-batch-2026-08-30/README.md)记录 1000 成员投影从 `16765.257 ms` 降至 `362.631 ms`，约 `46.2x` | 该数字只代表本地 MySQL 的单轮 SQL 投影。补同提交、隔离拓扑的千人群端到端基准，记录 notify/pull、投递、Kafka lag 与 P99，才能填写端到端 P99。 | P1 |
| Kafka + Elasticsearch 权限感知全文检索 | 部分完成 | 独立 Indexer/Search、Kafka 投影、Alias/version、scope fail-closed、created/recalled/迟到编辑端到端 smoke；[搜索设计](../data/ELASTICSEARCH-SEARCH.md) | 启动受控 Search Service，执行可见/不可见会话、发送者、时间范围和重放场景；归档检索 workload、权限拒绝计数和查询 P99。 | P1 |

### 当前可安全使用的 IM 表述

```text
基于 gRPC、Kafka、Transactional Outbox 与幂等投影构建渐进式 IM 服务边界；
采用 Conversation Timeline、User Sync Timeline、Read Seq 与 Device Cursor 支持历史漫游和多端增量同步；
在本地 MySQL 8.4 的固定 SQL workload 中，将 1000 成员会话投影由 16.765 s 优化至 362.631 ms（46.2x）。
```

## 3. Dipole Agent

| 简历能力 | 当前状态 | 已有证据 | 写入 Claim 前的验收缺口 | 优先级 |
| --- | --- | --- | --- | --- |
| TypeScript Runtime、可信上下文、Capability 授权、MCP、审批 | 部分完成 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、mTLS Core Capability、默认关闭写能力；[v2 隔离 Shadow receipt](../../benchmarks/agent-mcp-approval-shadow-2026-09-01-v2/)已验证本地 MCP 单次 Tool/Artifact、重启去重、过期 readiness、mTLS identity denial，以及 denied/consumed/failed-replay grant 全部阻断新增 effect；Task Timeline 现可从 `waiting_approval` 事件进入 owner-scoped 审批页 | 归档真实 approval deny/HITL UI receipt、真实外部 MCP 受控只读服务及共享环境的回滚与观测 receipt。对外表述继续限定为隔离只读 Shadow。 | P0 |
| Temporal Durable Task、故障恢复、等待输入与 HITL | 部分完成 | Temporal workflow、approval/input 状态机、Memory promotion retry；Remote GPU read-shadow 已验证 Kafka 到 Task/Run/Artifact 的只读闭环；[Worker replacement receipt](../../benchmarks/agent-temporal-fault-2026-09-01/) 已验证 approval/input resume 的状态转换与精确副作用基数 | 在同一受控拓扑补 Worker replacement、Core restart 与 EventLedger lease 的联合故障演练，并完成 approval/HITL UI 的受控回滚与观测 receipt。 | P0 |
| Context、检索增强与分层 Memory | 部分完成 | Context Compiler、受限 conversation search、working/episodic/semantic/procedural/observational 策略、reviewed promotion | 建立版本化且脱敏或合成的多轮任务集，分别评估无检索、检索、Memory 三种条件；记录 evidence recall、无授权访问、token/cost 与结果质量。 | P1 |
| OpenTelemetry、Eval、任务成功率 | 部分完成 | OTel 设计、模型调用审计、五类 Eval harness、`reviewed_shadow` 窗口汇总与持久化 Run Trace 绑定；已归档隔离完成子集 [N=2](../../benchmarks/agent-shadow-eval-window-2026-09-01-n2/)；窗口采集以评审 manifest 集合 SHA-256 固定输入，最低样本阈值与实际样本数共同写入低敏集合回执 | 对同一固定、人工评审的任务集运行足量终态 Shadow 样本，归档样本量、成功率、五类通过率、失败分类和 Run Trace ID；缺失 Token 的失败调用现会生成 `token_metrics_unavailable` 分类。完成这些证据后才能填写任务成功率 `[XX]%`。 | P0 |

### 当前可安全使用的 Agent 表述

```text
基于 TypeScript、Temporal 与 MCP 构建事件驱动 Agent Runtime，以系统派生的 ExecutionContext 和 Capability Policy 限制工具范围；
通过 Temporal durable receipt、mTLS Core RPC 与 owner-reviewed Memory promotion 约束可恢复任务和长期记忆写入；
当前写 Capability 与外部 MCP 以默认关闭的只读 Shadow 路径进行隔离验证。
```

## 4. Claim-First 实施顺序

1. **P0：可靠性与 Agent 结果证据。** 建立统一 evidence receipt 契约，先完成消息故障矩阵、Temporal/HITL 故障矩阵和 Agent Eval harness；这三项决定“零重复副作用”和“任务成功率”能否落笔。
2. **P0：同步体验实证。** 在同一隔离拓扑下完成认证 Sync pull、cursor/reconnect 和 Web Sync 观察，补同步 P99。
3. **P1：数据面 P99。** 依次运行 Cassandra history/read-fallback 基准、Elasticsearch 权限检索基准和千人群端到端基准；每个结论独立归档，禁止互相外推。
4. **P1：受控 Agent Shadow。** 将 Runtime Capability、MCP、approval、Temporal 状态与 OTel trace 放入单一受控 Shadow 演练，作为独立 Agent 项目的体验闭环。

当前阶段不以 C++ 替换或 UI 扩展作为主目标；它们不阻塞上述 claim 的证据闭环。

## 5. 更新检查

当任一 P0/P1 项完成时：

1. 将原始报告提交至 `benchmarks/` 或版本化 `contracts/agent-evals/`。
2. 更新本矩阵状态、两份项目材料、[性能基线](../performance/PERFORMANCE-BASELINE.md)与架构债务台账。
3. 只在报告包含明确环境和样本量后填写 `[XX]` 数字；否则保留占位符。
