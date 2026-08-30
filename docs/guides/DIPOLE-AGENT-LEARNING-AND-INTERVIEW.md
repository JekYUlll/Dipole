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
| 交互式 Agent Task 创建 | 已验证的默认关闭 API seam | Gateway JWT principal、确定性 Task ID、Remote GPU 定向回归 |
| reviewed Memory receipt、mTLS、MySQL retry | 已验证（隔离 Remote GPU） | `scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh` |
| External MCP Shadow 完整链路 | 已验证（隔离 Remote GPU） | `scripts/drill-agent-external-mcp-shadow.sh` |
| `conversation.search` 受控检索契约 | 已验证（Core/Proto/TS 与隔离 Remote GPU） | `internal/services/agent/application/agent_capability.go` |
| `promotion_active` 与 External MCP Shadow mode | 默认关闭 | [External MCP 运行手册](../agent/agent-external-mcp.md) |
| Project Guardian 预筛评测基线 | 已验证（合成离线） | `contracts/agent-evals/v1/project-guardian-synthetic-corpus.json` |

#### Project Guardian Evaluation Baseline

- **状态：** 已验证（合成离线）
- **简历句：** 为 IM-native Project Guardian 建立版本化事件预筛语料，覆盖承诺、决策、风险和缺失负责人等关注状态；双 reviewer agreement 与复用生产订阅 matcher 的 deterministic evaluator 固定回归基线。
- **演示：** 运行 `npm run eval:prefilter-review -- --corpus=../../contracts/agent-evals/v1/project-guardian-synthetic-corpus.json --review=../../contracts/agent-evals/v1/project-guardian-synthetic-review.json`；展示 hash-bound 的 agreement 报告和规则候选评测。
- **证据：** `services/agent-runtime/src/evals/project-guardian-synthetic-corpus.test.ts`、[Agent Evaluation Contract](../../contracts/agent-evals/v1/README.md)。
- **追问：** “为什么仍然不启用 subscription Runtime？” 当前是低敏 synthetic baseline，只说明评测协议、回归数据与门禁可重复；真实用户语料、候选模型效果、成本阈值和共享 shadow 观察需单独取证。
- **限制：** 不含真实消息、身份、任务执行、模型输出或在线流量，不能作为 production accuracy、成本或 active authority 的表述依据。
- **下一步：** 通过受控 owner approval 归档真实 Project Guardian corpus 和 retrieval relevance，再依次完成离线候选、shadow 观察及灰度回滚证据。

#### Durable Memory Promotion

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 设计 Agent Memory promotion 的 durable receipt 合约：Temporal Activity 经 mTLS 调用 Core，首次持久提交后重试仍收敛至同一 MySQL Memory，并在 grant 撤销后拒绝后续提交。
- **对外表述：** Runtime 只提交低敏 receipt；Core 每次从持久 Task/Run 恢复主体并复核有效 grant、candidate/review 与类型，避免模型或环境变量直接授予写权限。
- **演示：** 使用显式 Node 22 与 Go 1.27 运行 `scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh`，观察受控 Activity 故障重试、grant 撤销拒绝和 owner-scoped Memory revoke。
- **证据：** `services/agent-runtime/src/temporal/agent-memory-promotion-mtls-mysql.integration.test.ts`、`internal/services/agent/infrastructure/mysql/agent_memory_promotion_temporal_fixture_test.go`、[Memory promotion 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)。
- **追问：** “为什么 grant 在 admission 和 commit 都要复核？” 两个阶段可能跨越较长时间，撤销后旧 Run 仍须停止写入。
- **限制：** Temporal 使用内存 test server，MySQL/证书/监听器为临时资源；Gateway 到 Core 的 owner revoke HTTP/gRPC/mTLS 契约已覆盖，但 Kafka trigger、共享环境运行记录与 overlay 回滚仍待联合验证。
- **下一步：** 在受控共享环境补齐 Kafka trigger、Gateway-to-Core revoke 运行记录、overlay 回滚与观测窗口证据。
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

#### Mediated Conversation Search

- **状态：** 已验证的契约，默认关闭的 Shadow Context 编排
- **简历句：** 设计由 Core 调解的 `conversation.search` Capability：Runtime 只提交 Task/Run 和有界 query，Core 恢复权威主体后以独立 permission 与 `conversation/*/read` scope 访问检索端口；当前消息可在独立开关下触发有界检索，命中按预算降级为 `untrusted` evidence。
- **对外表述：** Search Service 的直连服务身份只授予业务网关；Agent 不能伪造 principal。Core 复核 Task/Run、权限和资源范围，再限制 query、返回数量与正文长度，避免检索结果覆盖策略或身份上下文。
- **工程取舍：** 会话、Memory 与检索均为独立的只读授权请求，Context hydration 并行发起以降低单轮等待；任何一个来源失败都会在模型调用前终止，不能以局部 evidence 降级继续推理。
- **演示：** 运行 `CGO_ENABLED=0 go test ./internal/application ./internal/services/agent/application ./internal/transport/grpc/agent` 与 `npm --prefix services/agent-runtime test -- --run src/capabilities/conversation-search.test.ts src/capabilities/agent-capability-rpc.test.ts src/models/model-shadow-planner.test.ts`，展示 forged principal、窄 scope、缺少 Search port、检索失败的模型前拒绝与 `untrusted` Context provenance。
- **证据：** `api/proto/dipole/agent/v1/agent.proto`、`internal/transport/grpc/agent/server.go`、`services/agent-runtime/src/capabilities/conversation-search.ts`、[架构参考](../architecture/architecture-reference.md)。
- **追问：** “为何不让 Agent Runtime 直连 Elasticsearch？” 服务级凭据无法表达单次 Task 的 owner、授权状态与资源 scope，Runtime 直连会把这一边界交给调用方。Core 代管后能复用持久 invocation resolver，并在 RPC 入口拒绝客户端传入 principal。
- **限制：** Core-to-Search assembly、Runtime registry 与 retrieval-to-Context 编排均默认关闭；生产 Elasticsearch、跨会话灰度、向量检索与多轮 retrieval orchestration 仍关闭。
- **下一步：** 归档同版本 Shadow 观测与 operator evidence，再审阅 retrieval-to-Context 的受控开关；随后才评估多轮检索和向量检索。
- **复核条件：** 修改 Search caller allowlist、Agent permission/scope、Task/Run resolver、evidence 上限、Context Compiler 或 Runtime composition 时。

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

早期版本将 AI 作为特殊用户，经 `message.direct.created` 消费、最近会话上下文和 Eino tool calling 回写普通消息，`ai_call_logs.trigger_message_uuid` 用唯一键收敛重复触发。它证明了 IM 事件接入、基础 Tool 和可观测性；独立 TypeScript Runtime 将可信身份、Capability、Temporal 状态和长期 Memory 从这条单聊消费链中分离出来。旧实现保留为 embedded 回滚基线，服务布局门禁限制它只能由 embedded Kafka composition 引用，独立服务入口不可接入该链路；不能表述为当前 active Agent Runtime 的权限模型。

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
