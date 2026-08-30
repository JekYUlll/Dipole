# Dipole 项目学习与面试主文档

本文档是 Dipole 的简历、现场介绍和复盘入口。内容必须以代码、测试、基准报告和架构文档为依据；详细题库见 [面试问答](INTERVIEW-QA.md)。

## 1. 使用规则

每次新增可对外描述的能力时，更新以下四项：

1. 简历描述与现场介绍。
2. 对应的证据链接与状态。
3. 至少一个可追问的问题。
4. 已知限制与下一步学习方向。

状态标签：

- **已验证**：有实现和可复核测试、Smoke 或性能证据。
- **默认关闭**：实现与门禁存在，生产切流仍缺共享环境或审批证据。
- **规划中**：只记录设计方向，不能写入简历成果。

## 2. 一句话定位

Dipole 是一个面向实时协作与 Agent 能力演进的现代 IM 平台：Go 承担 IM 领域与一致性边界，Kafka 解耦事件与投影，TypeScript Runtime 承担可恢复 Agent Task，并以渐进式微服务化和可回滚切换替代一次性重写。

## 3. 简历描述

### 后端 / 分布式系统版本

```text
Dipole 现代 IM 平台 | Go, sqlc, MySQL, Kafka, Redis, Cassandra, Elasticsearch, MinIO, WebSocket
- 设计消息幂等、Transactional Outbox 与 Kafka 事件链路，按 Message、Conversation 和 Sync Timeline 分离事实存储、用户会话状态与多端增量同步。
- 将 Core、Gateway、Message、Sync、Search 抽象为可独立部署的服务边界，保留 embedded 兼容路径，并以 gRPC、版本化契约、Shadow、回滚和故障演练推进迁移。
- 面向热点群采用 notify + pull 与增量 Timeline；基准记录 100 成员场景的投递、Kafka lag、Inbox 写放大和端到端延迟，性能结论均以归档报告为准。
```

### Agent / AI 工程版本

```text
Dipole Agent Runtime | TypeScript, Node.js, Temporal, Kafka, MCP, OpenTelemetry
- 构建事件驱动的 Agent Task Runtime，包含可信 ExecutionContext、Capability Registry、Context Compiler、Memory 策略、Temporal Workflow 与人工审批状态。
- 以 Provider 注入、模型调用审计、结构化输出、预算限制和五类 Eval 管理模型路径；MCP、写 Capability 和 active authority 按默认关闭与显式 promotion 证据控制。
- 设计 OpenAI-compatible Provider、独立 consumer group、user-gray Active Compose profile 与可执行回滚，避免模型、Tool 和部署配置绕过权限边界。
```

### 使用边界

简历中可使用“已设计并实现”“已通过本地/隔离环境验证”等准确表述。不要把 Cassandra 主读、Elasticsearch 默认搜索、Agent active authority、外部 MCP 写入或 C++ 数据面性能收益写成已上线成果，详见 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md)。

## 4. 现场介绍

### 60 秒版本

Dipole 是我持续迭代的现代 IM 项目。核心消息链路由 Go 服务负责，通过 Kafka 和 Transactional Outbox 将消息持久化、会话投影、用户同步和实时投递解耦。数据模型把消息历史、用户会话状态和多端 Sync Timeline 分开，使用会话序列和设备 cursor 支持增量同步。项目从模块化单体出发，逐步形成 Core、Gateway、Message、Sync、Search 与 TypeScript Agent Runtime 的服务边界，并为每次切换保留 Shadow、回滚和验证门禁。Agent 部分强调可信上下文、Capability 权限、Temporal 可恢复任务、Memory 和 MCP 安全边界。

### 3 分钟版本

先从 IM 数据模型讲：消息事实按会话 Timeline 存储，Conversation 提供用户视角的摘要和已读状态，Sync Inbox 为每个用户提供可单调推进的同步流。这样历史查询、首页状态和多端增量同步各自有清晰责任。

消息发送经过鉴权、幂等校验、持久化和 Outbox；Kafka 事件再驱动会话、Sync、搜索、实时投递和 Agent 投影。群聊区分普通群和热点群，热点群使用 notify + pull 降低扇出压力。微服务化采用渐进迁移，先稳定接口、数据所有权和 gRPC 契约，再抽离部署单元，embedded 路径用于回滚。

最后是 Agent：Runtime 独立为 TypeScript 服务，通过 Capability RPC 使用 IM 能力，模型无法自行指定用户身份或资源范围。Temporal 管理长任务和审批等待，MCP 与写操作保持默认关闭，active 需要可复核的评测、release manifest、权限和共享环境证据。我的重点是把可靠性、权限和可观测性放在 Agent loop 外层，而非只实现一次模型调用。

## 5. 可展开的工程故事

| 主题 | 可讲的取舍 | 证据与深入材料 |
| --- | --- | --- |
| Sync Timeline | 将历史、会话状态和用户同步流拆开，避免用 `messages.id` 同时承担所有语义 | [消息存储与同步模型](../architecture/MESSAGE-STORAGE-AND-SYNC.md)、[Sync Service](../architecture/SYNC-SERVICE.md) |
| 可靠消息 | 通过 Outbox 缩小“已落库但未发布”缺口，consumer 依赖幂等和重试边界 | [Kafka 事件契约](../data/KAFKA-EVENT-CONTRACT.md)、[面试问答](INTERVIEW-QA.md) |
| 热点群 | 在完整 push 与 notify + pull 之间按负载切换，接受客户端补拉复杂度以控制扇出 | [Realtime Delivery](../architecture/REALTIME-DELIVERY.md)、[性能基线](../performance/PERFORMANCE-BASELINE.md) |
| 渐进微服务 | 接口、契约、Shadow、独立入口和 embedded 回滚并存，降低一次性拆分风险 | [服务边界](../architecture/SERVICE-BOUNDARIES.md)、[微服务部署](../architecture/MICROSERVICES-DEPLOYMENT.md) |
| Agent 安全 | ExecutionContext 和 Capability policy 位于模型外层，Tool 与权限分离 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[MCP 授权](../agent/agent-mcp-authorization.md) |
| Agent 可恢复执行 | Temporal 保存任务状态，人工输入与审批可恢复；模型输出仍受审计和预算约束 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[Active 部署运行手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md) |

## 6. 高频追问

### 为什么没有直接把每个模块拆成独立服务？

服务数量不会自动改善消息可靠性或数据所有权。Dipole 先用模块边界和接口隔离稳定语义，再将热点和独立运行需求明确的模块抽离。这样可以保留可测试的本地组合和 embedded 回滚路径，降低迁移期间的故障定位成本。

### Kafka 能否直接作为用户离线消息同步队列？

Kafka 用于内部事件传播和投影。用户同步需要面向用户与设备的长期域状态、稳定 cursor、权限重算和重连语义，因此由 Sync Timeline 承担。两者通过事件连接，消费组 offset 不承担客户端同步协议。

### 为什么 Message ID、Conversation Seq 和 Sync Seq 要分开？

全局 Message ID 用于唯一性和幂等；Conversation Seq 表达会话内顺序；Sync Seq 表达某个用户的增量消费顺序。三者的分区、排序和重试范围不同，混用会增加分页、已读、多端和迁移的复杂度。

### Agent 为什么选择 TypeScript？

Agent 主要是模型、工具、工作流与协议集成，TypeScript 对 Zod、JSON Schema、MCP、Node I/O 和 Temporal SDK 的支持较完整。Go 继续负责 IM 领域约束与一致性，语言职责按数据面、控制面和智能执行面拆分，避免模型 Runtime 直接接触业务存储。

### Agent active 为什么仍然默认关闭？

模型调用、工具权限和长期任务会引入成本、数据访问和副作用风险。当前 active profile 只允许只读 Temporal Activity，并要求 user-gray manifest、五类 Eval、Operator grant、共享 Kafka/Temporal/RPC/Provider 证据与维护窗口。缺少任一证据时保持 Shadow。

更多网络、存储、性能、SQLC、MCP、C++ 与故障恢复问题见 [详细面试问答](INTERVIEW-QA.md)。

## 7. 学习路线

| 阶段 | 目标 | 建议练习 | 完成标志 |
| --- | --- | --- | --- |
| IM 基础 | 讲清消息、会话、未读、已读和 Sync Timeline | 手画数据流与 cursor 推进过程 | 能解释三个序列的边界 |
| 可靠性 | 掌握幂等、Outbox、重试、DLQ 和投影一致性 | 演示重复事件和发布失败如何收敛 | 能描述失败模式与回滚 |
| 微服务 | 理解服务边界、RPC、配置和数据所有权 | 用一条消息追踪 Gateway 到投影 | 能说明为何渐进拆分 |
| 性能 | 基于报告解释 bottleneck，避免孤立指标结论 | 比较普通群与热点群证据 | 能区分吞吐、延迟、写放大和 lag |
| Agent | 掌握 ExecutionContext、Tool policy、Memory、Temporal 和 Eval | 演示等待审批后恢复的状态机 | 能解释模型不能决定权限 |
| 复盘 | 诚实说明已验证、默认关闭与规划能力 | 每次提交更新本节与题库 | 对局限有明确下一步 |

## 8. 面试前检查

1. 从 [README](../../README.md) 复核当前架构与服务列表。
2. 从 [更新日志](../../CHANGELOG.md) 选择最近两个有测试证据的改动。
3. 从 [性能基线](../performance/PERFORMANCE-BASELINE.md) 选择一组结果，并说明其环境与局限。
4. 从 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 选择一个未完成项，准备解释风险和计划。
5. 使用 60 秒和 3 分钟版本各练习一次，再从详细题库抽取 5 个追问。
