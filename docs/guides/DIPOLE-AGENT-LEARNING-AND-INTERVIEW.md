# Dipole Agent 项目学习、简历与面试

本文件将 Dipole Agent Runtime 作为独立 AI 工程项目描述。IM 数据面和同步能力请使用 [Dipole IM 项目材料](DIPOLE-IM-LEARNING-AND-INTERVIEW.md)。

## 1. 使用规则

Agent 口径必须区分已验证、默认关闭和规划中。模型、MCP、Memory 或 active 写入的实现不能替代权限、评测、可观测性和共享环境证据。

### 滚动维护契约

ExecutionContext、Capability、Temporal、Memory、MCP、评测、运行模式或权限边界变化时，同步更新本文件。协议与运行事实以 [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md) 和架构债务台账为准。

### 能力卡片模板与索引

| 能力 | 状态 | 证据 |
| --- | --- | --- |
| ExecutionContext、Capability Policy、Temporal Task | 已验证 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md) |
| reviewed Memory receipt、mTLS、MySQL retry | 已验证（隔离 Remote GPU） | `scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh` |
| External MCP Shadow 完整链路 | 已验证（隔离 Remote GPU） | `scripts/drill-agent-external-mcp-shadow.sh` |
| `promotion_active` 与外部 MCP | 默认关闭 | [Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md) |

#### Durable Memory Promotion

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 设计 Agent Memory promotion 的 durable receipt 合约：Temporal Activity 经 mTLS 调用 Core，首次持久提交后重试仍收敛至同一 MySQL Memory，并在 grant 撤销后拒绝后续提交。
- **对外表述：** Runtime 只提交低敏 receipt；Core 每次从持久 Task/Run 恢复主体并复核有效 grant、candidate/review 与类型，避免模型或环境变量直接授予写权限。
- **演示：** 使用显式 Node 22 与 Go 1.27 运行 `scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh`，观察受控 Activity 故障重试、grant 撤销拒绝和 owner-scoped Memory revoke。
- **证据：** `services/agent-runtime/src/temporal/agent-memory-promotion-mtls-mysql.integration.test.ts`、`internal/services/agent/infrastructure/mysql/agent_memory_promotion_temporal_fixture_test.go`、[Memory promotion 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)。
- **追问：** “为什么 grant 在 admission 和 commit 都要复核？” 两个阶段可能跨越较长时间，撤销后旧 Run 仍须停止写入。
- **限制：** Temporal 使用内存 test server，MySQL/证书/监听器为临时资源；Kafka trigger、Gateway owner revoke 的网络传输与共享环境 overlay 回滚仍待联合验证。
- **下一步：** 在受控共享环境补齐 Kafka trigger、Gateway owner revoke 传输、overlay 回滚与观测窗口证据。
- **复核条件：** 修改 receipt canonicalization、grant、Temporal retry、Core caller policy、Memory schema 或 mTLS 时。

#### External MCP Shadow Drill

- **简历句：** 为外部 MCP 只读调用构建可释放的 Shadow 验证链，串联 Kafka、MySQL EventLedger、Temporal、mTLS Core RPC 与受限 MCP Tool，并用重启重放和过期 readiness 验证安全收敛。
- **演示：** 运行 `scripts/drill-agent-external-mcp-shadow.sh`；查看低敏 evidence 中的事件数、Tool/Artifact 数、重启去重与 readiness 拒绝结果。
- **证据：** [外部 MCP 运行手册](../agent/agent-external-mcp.md)、`services/agent-runtime/src/runtime/external-mcp-full-stack-drill.integration.test.ts`、`contracts/agent-external-mcp/v2/shadow-drill-evidence.schema.json`。
- **追问：** “为什么重发相同事件不能重复调用 Tool？” Kafka 至少一次投递和 Runtime 重启会产生重复输入，持久 EventLedger 与稳定 Task ID 共同限制只执行一次。
- **限制：** 演练使用本地 MCP fixture、临时 CA、临时 MySQL/Kafka 与内存 Temporal；未接入共享身份、外部 DNS/TLS、凭据轮换或生产 authority。
- **下一步：** 在独立 Shadow tenant 使用受控只读 Server，补齐真实 Provider owner、凭据吊销、网络故障和观测窗口证据。
- **复核条件：** 修改 EventLedger、Kafka group、Temporal route、Core RPC、MCP transport 或 readiness policy 时。

#### Agent Active Boundary

- **状态：** 默认关闭
- **简历句：** 以 ExecutionContext、Capability Registry、Context Compiler、Temporal 状态机与分层 Memory 构建 TypeScript Agent Runtime，并用显式 promotion 门禁约束 active 写入。
- **对外表述：** 模型只能选择受声明 Tool；主体、租户、资源范围和审批来自系统认证上下文。长任务通过 Temporal 持久化等待、恢复与取消。
- **演示：** 展示 Shadow Task 的低敏轨迹，或运行 Temporal approval fixture；不要在无授权环境启用写 Capability。
- **证据：** [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[MCP 授权](../agent/agent-mcp-authorization.md)、[Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)。
- **追问：** “为什么 Agent 选 TypeScript？” Zod/JSON Schema、MCP、Node I/O、流式协议与 Temporal SDK 的组合适合 Runtime 集成，Go 继续承担 IM 领域一致性。
- **限制：** active profile 与外部 MCP 仍默认关闭，不能描述为生产自动写入能力。
- **下一步：** active Kafka consumer 现要求独立 `dipole-agent-active-*` group，并已验证消息可进入 Temporal dispatcher 合约；继续以同一 candidate 的评测、release manifest、operator grant 与共享环境证据完成受控灰度。
- **复核条件：** 修改 Capability schema、provider、模型 route、Temporal queue、MCP transport 或权限策略时。

## 2. 一句话定位

Dipole Agent 是一个事件驱动的 TypeScript Agent Runtime：以可信 ExecutionContext、Capability Policy、Temporal durable task、Context Compiler 与 Memory 机制，将模型调用放入可审计、可恢复、可授权的系统边界内。

## 3. 简历描述

```text
Dipole Agent Runtime | TypeScript, Node.js, Temporal, Kafka, MCP, OpenTelemetry
- 构建事件驱动的 Agent Task Runtime，包含可信 ExecutionContext、Capability Registry、Context Compiler、分层 Memory 和 Temporal 审批状态机。
- 设计 receipt 驱动的 Memory promotion：Core 从持久 Task/Run 恢复身份并复核 grant、candidate/review；隔离演练验证 mTLS、MySQL 幂等重试、撤销后拒绝与 owner-scoped revoke。
- 通过 Provider 注入、模型审计、预算、结构化输出和 Eval 管理模型路径；写 Capability 与外部 MCP 保持默认关闭并受 promotion 证据门禁约束。
```

## 4. 现场介绍

### 60 秒版本

Dipole Agent 将模型视为不可信的推理组件。Runtime 用系统生成的 ExecutionContext 固定主体和资源范围，Capability Registry 决定可用 Tool，Temporal 负责长任务、审批、等待和恢复。Memory promotion 用低敏 receipt 连接 TypeScript、Core 与 MySQL，每次提交都复核 Task/Run、grant 与 owner-reviewed candidate。

### 3 分钟版本

入口事件先被转换为可信 Task，再由 Context Compiler 按预算组合策略、会话证据、Memory 与 Tool schema。模型只能调用已注册 Capability，写操作需要审批和 Core 侧复核。Temporal 将任务状态和人工输入持久化，Worker 重试必须使用稳定 receipt。Memory 长期化需要 candidate/review，Core 从持久 invocation 恢复 owner。隔离联合演练已证明 mTLS 传输、MySQL 幂等、grant 撤销拒绝和 owner revoke；Kafka trigger 与共享环境灰度仍保持关闭。

## 5. 可展开的工程故事

| 主题 | 取舍 |
| --- | --- |
| 可信上下文 | 模型不传 principal/tenant/scope，系统从认证与 Task/Run 派生。 |
| Capability | Tool 是 Capability 的受限投影，风险与权限在模型循环外执行。 |
| Durable Task | Temporal 处理等待、重试和恢复，副作用用 receipt 与幂等绑定收敛。 |
| Memory | working 仅任务内使用；长期类型需要 owner-reviewed candidate。 |
| 默认关闭 | active、写 Capability 与外部 MCP 需要可复核评测、授权和运行证据。 |

## 6. 高频追问

### 旧 Eino 实现如何演进到独立 Runtime？

早期版本将 AI 作为特殊用户，经 `message.direct.created` 消费、最近会话上下文和 Eino tool calling 回写普通消息，`ai_call_logs.trigger_message_uuid` 用唯一键收敛重复触发。它证明了 IM 事件接入、基础 Tool 和可观测性；独立 TypeScript Runtime 将可信身份、Capability、Temporal 状态和长期 Memory 从这条单聊消费链中分离出来。旧实现是迁移基线，不能表述为当前 active Agent Runtime 的权限模型。

### 为什么 receipt 不能直接等于授权？

receipt 只绑定完整性和幂等信息。Core 仍需每次从持久 Task/Run 恢复身份，并查询当前有效 grant 与 candidate/review。

### 为什么 active 写入默认关闭？

模型调用会引入数据访问、副作用和成本风险。共享 Kafka、Temporal、RPC、Provider、评测与 operator grant 缺少任一证据时，Runtime 保持 Shadow 或只读边界。

### 为什么不把 Agent 逻辑放回 Go 服务？

IM 领域一致性继续由 Go 管理；TypeScript Runtime 更适合模型、Schema、MCP、流式 I/O 和 Temporal 编排。跨语言以 protobuf 和 Capability contract 协作。

## 7. 学习路线

1. 画出 Event 到 Task、Context、Model、Tool、Approval、Artifact 的链路。
2. 用一个审批等待场景说明 Temporal Signal、Timer 和恢复。
3. 用 Memory receipt 解释身份恢复、grant 撤销和幂等重试。
4. 明确区分隔离联合演练与共享环境 active 证据。

## 8. 面试前检查

1. 复核 [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md) 与 [Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)。
2. 练习一次 60 秒介绍和一次 receipt retry/grant revocation 故事。
3. 从 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 选择一个默认关闭边界，说明启用条件与回退。
