# Dipole 项目学习、简历与面试主文档

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

### 滚动维护契约

本文件是学习、答辩和面试叙事的主入口。架构设计、运行手册、测试报告、设计稿和更新日志保留各自的事实细节；本文件只汇总可讲结论、证据链接、追问和限制，避免复制后发生漂移。

| 触发项 | 本文档必须同步的内容 | 可引用的事实源 |
| --- | --- | --- |
| 新增或改变服务/数据边界 | 简历描述、60 秒/3 分钟介绍、至少一个取舍问答 | 架构计划、服务边界、契约与 migration |
| 新增用户可见流程或 Pencil 基线 | 产品演示步骤、界面状态和视觉回归证据 | `design/`、前端设计计划、Playwright 用例 |
| 新增 Agent 能力或权限状态 | Capability 边界、审批链与默认开关 | Agent Runtime 设计、授权与部署手册 |
| 取得性能、远程或故障演练结果 | 可复现环境、指标、适用范围和限制 | 基准报告、运行记录、更新日志 |
| 切换默认路径或发现风险 | 状态标签、限制和下一步 | 架构债务台账、回滚手册 |

每个合并切片至少复核本文档是否受影响；若无变化，在切片的测试/合并记录中注明“面试叙事无变化”。所有描述继续遵守证据优先：实现与测试齐备才标记为“已验证”，部署门禁齐备但缺共享环境证据标记为“默认关闭”，设计或待验收内容标记为“规划中”。

### 合并时的更新记录

每个会改变项目定位、可演示能力、验证状态或技术取舍的合并切片，都在本文档对应能力卡片或新增能力卡片中追加一条简短更新记录。记录采用以下格式，日期使用切片合并日：

```md
#### YYYY-MM-DD · <能力名称>

- **状态：** 已验证 / 默认关闭 / 规划中
- **对外表述：** 可放入简历或现场介绍的一句话。
- **演示：** 受控环境中的最短复现步骤。
- **证据：** 实现、测试、基准、运行记录或设计稿链接。
- **追问：** 一个可展开的工程问题及回答入口。
- **限制：** 当前证据未覆盖的边界。
- **复核条件：** 下次必须重新核验的开关、环境或指标。
```

`README.md` 只保留项目定位、启动和目录入口；`CHANGELOG.md` 只记录时间线；架构文档、运行手册和测试报告保留可复核细节。本文档负责把这些事实组织为可讲、可演示、可追问的叙事，不复制实现细节。若文档之间发生冲突，以代码、契约、测试和归档运行记录为准，并在下一次合并中修正叙事。

### 能力卡片模板与索引

每个可讲的合并切片在本文档新增或更新一张能力卡片，固定保留以下字段：`简历句`、`现场演示`、`证据`、`追问`、`限制` 和 `下一步`。更新日志记录变更时间线，能力卡片只保留可复述、可核验的当前结论。

| 能力卡片 | 状态 | 简历句与现场演示入口 | 证据与追问 |
| --- | --- | --- | --- |
| 实时 IM 与 Timeline | **已验证** | 第 3 节后端描述；第 4 节 60 秒/3 分钟介绍 | [消息存储与同步模型](../architecture/MESSAGE-STORAGE-AND-SYNC.md)；“三个 Seq 为什么分开？” |
| 渐进式微服务与 SQLC | **已验证** | 第 3 节后端描述；第 5 节渐进微服务故事 | [服务边界](../architecture/SERVICE-BOUNDARIES.md)；“为什么不一次性拆分？” |
| Agent Runtime 与权限 | **已验证** | 第 3 节 Agent 描述；第 5 节 Agent 安全与可恢复执行 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)；“模型为何不能决定权限？” |
| Owner-reviewed Memory 类型晋级 | **已验证（本地）** | 受控 candidate/review 选择 semantic 等持久类型 | [Memory promotion 契约](../../contracts/agent-memory-promotion/v1/README.md)；“为何 working 不能晋级？” |
| Agent Definition Catalog | **已验证（本地）** | 只读目录演示：版本、scope 和 runtime 关闭边界 | `frontend/src/components/AgentDefinitionCatalog.vue`、`frontend/e2e/agent-definitions.spec.ts`、`frontend/e2e/agent-definitions.visual.spec.ts`；认证流程已通过 Chromium/Firefox/WebKit，视觉基线仅固定 Chromium；“为何 Definition 目录不提供激活或编辑？” |
| Artifact 与 Task Timeline 关联 | **已验证（本地）** | Timeline `artifact` 事件以内容寻址 ID 打开 owner-scoped metadata 页面，并固定正文与下载关闭边界 | [Timeline 契约](../../contracts/agent-task-timeline/v1/README.md)、`frontend/src/components/AgentArtifactMetadata.vue`、`frontend/e2e/agent-artifact.spec.ts`；认证读取已通过 Chromium/Firefox/WebKit，视觉基线仅固定 Chromium；“为什么 Timeline 只返回 Artifact ID？” |
| Active Agent、外部 MCP 与 C++ 数据面 | **默认关闭 / 规划中** | 仅展示门禁、Shadow 与回滚设计，不作为上线能力演示 | [架构债务台账](../architecture/ARCHITECTURE-DEBT.md)；“何时允许切流？” |

#### 2026-08-30 · Owner-reviewed Memory 类型晋级

- **状态：** 已验证（本地）
- **对外表述：** 将 Agent Memory 的类型策略从 Runtime 校验延伸至持久化事务：owner 在已接受 review 后可将 observational candidate 晋级为 semantic、episodic、procedural 或 observational Memory，并由 Gateway、gRPC、Core 和 MySQL 共同校验。
- **演示：** 使用受控 candidate/review 调用 promotion RPC，指定 `semantic` 后读取返回 Memory 类型；再提交 `working`，确认 Gateway 返回 400 且未触发写入。
- **证据：** [Memory promotion 契约](../../contracts/agent-memory-promotion/v1/README.md)、`internal/services/agent/application/agent_memory_candidate_promotion_test.go`、`internal/transport/grpc/agent/server_test.go`、`internal/services/gateway/server/server_test.go`。
- **追问：** “为什么 working 不能晋级？” working 只服务当前 Task 的短期推理状态，持久化会扩大生命周期和检索范围；长期 Memory 必须经过 owner review，并在事务内绑定 candidate 与 review。
- **限制：** 当前路径使用 owner 控制 RPC；TS receipt v2 的短时效、Core commit service 和独立 active executor 契约已固定，但 Core RPC、Temporal Activity、授权演练和共享环境证据尚未接入，不能宣称 active Agent 已自动写入长期 Memory。
- **复核条件：** 接入 receipt、Temporal Activity、active authority 或增加新的 Memory 类型时。

#### 2026-08-30 · Artifact 与 Task Timeline 关联

- **状态：** 已验证（本地）
- **对外表述：** 为 Agent Task Timeline 增加内容寻址 Artifact 关联，并让主投影与失败修复队列共享同一持久化契约，保证重试后仍可回到相同 Artifact metadata 边界。
- **演示：** 使用受控 Artifact 创建事件读取 Timeline，确认 `kind=artifact` 返回 64 位 `artifact_id`；再通过 owner-scoped metadata API 读取低敏元数据。
- **证据：** [Timeline 契约](../../contracts/agent-task-timeline/v1/README.md)、[Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、`internal/transport/grpc/agent/server_test.go`、`services/agent-runtime/src/capabilities/agent-capability-rpc.test.ts`。
- **追问：** “为什么不在 Timeline 直接返回正文或对象键？” Timeline 是低敏执行索引，正文读取需要独立披露策略、对象访问授权和前端设计，避免时间线接口扩大数据暴露范围。
- **限制：** Artifact metadata 页面仅在默认关闭的 flag 下通过本地组件、三浏览器认证读取、Chromium fixture 和设计导出验证；正文、下载、跨浏览器视觉证据和共享环境运行记录尚未完成。
- **复核条件：** 启用 Artifact Web 页面、正文读取或下载、修改对象生命周期，或改变 Timeline 事件 schema 时。

#### 2026-08-30 · Artifact 只读 Metadata 页面

- **状态：** 已验证（本地）
- **对外表述：** 为内容寻址 Artifact 增加 owner-scoped 的只读 metadata 展示：Timeline 只在 Artifact event 上跳转，页面复核 SHA-256、Task/Run 与低敏元数据，并将正文和下载明确保持关闭。
- **演示：** 使用受控 Timeline Artifact event 打开 metadata 页面，确认只显示类型、版本、大小、Task/Run、创建时间与摘要；模拟读取失败，确认旧 metadata 被清空并只提供重试。
- **证据：** [Pencil 设计说明](../../design/README.md)、`frontend/src/api/agentArtifacts.test.ts`、`frontend/src/components/AgentArtifactMetadata.test.ts`、`frontend/e2e/agent-artifact.spec.ts`。
- **追问：** “为何不直接给 Artifact 加下载链接？” 下载需要独立的对象访问授权、审计和披露策略；当前读取页只承担低敏发现，避免 Timeline 或 metadata API 扩大对象访问面。
- **限制：** 三浏览器功能验证与 Chromium 截图均不能代表共享环境、跨浏览器像素级视觉或任何正文读取能力。
- **复核条件：** 改变 metadata schema、Feature Flag、Timeline 关联，或评审正文/下载授权时。

#### 2026-08-30 · 学习、简历与面试叙事维护

- **状态：** 已验证
- **对外表述：** 为持续演进的分布式 IM 项目维护证据驱动的简历、演示与追问主文档，使能力陈述和验证边界可以随每次架构切片复核。
- **演示：** 从根 README 进入本文档，选择一张能力卡片，沿证据链接复核实现、测试或运行手册，再使用 60 秒与 3 分钟版本介绍该能力。
- **证据：** [根 README](../../README.md)、[文档目录](../README.md)、[更新日志](../../CHANGELOG.md)、[架构债务台账](../architecture/ARCHITECTURE-DEBT.md)。
- **追问：** “如何避免简历描述超过真实验证范围？” 通过状态标签、证据链接、限制项和合并复核，将默认关闭与规划能力从已验证成果中分离。
- **限制：** 文档维护无法替代共享环境运行、压测或生产授权证据；状态需随默认开关和运行结果重新核验。
- **复核条件：** 每个改变服务边界、默认路径、用户可见流程、性能结论或 Agent 权限的合并切片。

能力卡片的现场演示必须使用受控 fixture 或隔离环境；涉及真实消息、外部 MCP、生产凭据和写 Capability 时，先按对应运行手册完成授权与脱敏检查。

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

### 面试证据速查

| 叙事主题 | 当前状态 | 面试前应复核的证据 |
| --- | --- | --- |
| 服务边界与回滚 | **已验证** | [服务边界](../architecture/SERVICE-BOUNDARIES.md)、[微服务部署](../architecture/MICROSERVICES-DEPLOYMENT.md) 与对应 Smoke 记录 |
| SQLC 数据访问 | **已验证** | [数据访问迁移说明](../data/DATA-ACCESS-MIGRATION.md) 与版本化 migration/sqlc 查询 |
| Temporal 审批恢复 | **已验证** | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[MCP 授权](../agent/agent-mcp-authorization.md) 与 Workflow 回归测试 |
| Active Agent | **默认关闭** | [Active 部署运行手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)、release manifest、五类 Eval 与共享环境记录 |
| 外部 MCP Shadow | **默认关闭** | [外部 MCP 连接边界](../agent/agent-external-mcp.md)、`agent-external-mcp-shadow.yml`、Compose 门禁与隔离全栈演练；真实公网/凭据/共享环境证据仍需复核 |
| Cassandra/Elasticsearch 切流 | **默认关闭** | [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 中的回填、对账、Shadow 和回滚门禁 |
| C++ 实时数据面 | **规划中** | [平台演进计划](../architecture/PLATFORM-EVOLUTION-PLAN.md) 与基准报告；在可复现收益前不作性能承诺 |

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
| 设计到实现闭环 | Pencil canonical frame 定义只读 Timeline 的信息边界，Chromium snapshot 固定当前 Vue 页面，后续页面与跨浏览器基线独立推进 | [前端设计计划](../frontend/FRONTEND-DESIGN-PLAN.md)、[设计说明](../../design/README.md)、`frontend/e2e/agent-task-timeline.visual.spec.ts` |

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

### Temporal 等待审批时，如何避免错误用户批准了错误任务？

Workflow 先通过 Core 创建持久 Approval，再进入 `waiting_approval`。Signal 必须同时匹配当前 request 和 approval ID；Activity 会把 Task、Run、Runtime、审批 ID、决策和经 Gateway 认证的 actor 交给 Core。Core 重读持久 Task/Run，要求 actor 等于 Task principal，并用条件更新收敛首个 approved/denied 决策。重放只接受完全相同的已决结果，参数漂移、过期、撤销或跨 Task 引用都会拒绝。写 Tool 仍需后续的 grant 解析与原子 consume，因此“收到 Signal”本身不授予副作用权限。

### 为什么从 GORM 迁移到 SQLC？

消息、同步和投影的关键路径需要明确 SQL、索引、锁语义和跨服务可复用的数据契约。SQLC 让查询、参数和结果类型在编译期绑定，便于审查 MySQL 事务边界、迁移版本和最小权限授权；Go 服务继续把领域规则放在 application 层。这个选择也让后续多语言服务能够共享 protobuf、SQL schema 与数据库所有权约束，而不依赖某个语言的 ORM 行为。

### 远程部署和压力测试怎样避免“本地能跑”的伪证据？

开发环境区分 Remote GPU 的完整拓扑和 TencentCloud 的低资源 Smoke。每次运行应绑定 Git revision、镜像或源码摘要、配置摘要和资源快照，先通过 migration、readiness、mTLS、Kafka lag 与健康检查，再记录 P50/P95/P99、错误率和资源水位。活动会话受到保护时只进行只读审计或本地契约测试，不能用静态渲染、单元测试或未获批准的共享环境操作替代运行时证据。

### C++ 实时数据面为什么留到后期？

当前 Go Delivery 仍是权威路径。C++ 候选聚焦连接、批量投递、背压和节点级 fanout 等数据面工作；它需要在稳定协议之上用相同流量、同一指标和自动回切策略证明收益。这样语言边界有明确性能动机，也避免为了技术栈展示而把 CRUD 领域拆到 C++。

### 如何证明前端设计稿没有停留在静态展示？

设计以 canonical Pencil frame、共享 `--dp-*` token 和受控 Playwright fixture 形成三层闭环。以 Agent Task Timeline 为例，Pencil 定义 desktop/mobile/state matrix 和低敏 metadata 边界；Vue 路由与组件测试固定状态机映射；Chromium visual test 再固定当前只读页面的 revision、Capability、分页入口及 raw event kind 不直接呈现。该基线不代表所有页面、所有浏览器或 active Agent 已完成验收，剩余范围继续由前端计划和 AD-044 管理。

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
6. 对照最近一次合并切片的测试记录，确认本文件的状态标签、证据链接、限制和复核条件未过期。
