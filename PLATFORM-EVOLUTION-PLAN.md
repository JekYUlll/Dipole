# Dipole 三阶段演进计划

> 状态：计划中
>
> 基线：`99f2ef0 feat(sync): add user inbox timeline`
>
> 更新日期：2026-08-26

## 1. 目标

Dipole 按以下顺序完成三次独立演进：

1. **微服务改造：** 从模块化单体渐进拆出 Gateway、Message 和 Sync 服务，Core 暂时保留 User、Group、Contact、File、Auth。
2. **架构重构：** 建立 MySQL 元数据、Kafka 事件流、Cassandra 消息存储、Elasticsearch 搜索索引和 Redis 实时状态的分层架构。
3. **Agent 化：** 将进程内 Eino AI 模块演进为独立 Agent Runtime，通过事件和受控 Capability API 参与 IM 业务。

整个过程采用 Strangler Fig 和事件驱动抽离，任何阶段结束时都必须存在可部署、可测试、可回滚的版本。

## 2. 演进原则

- **一次只改变一个维度：** 模块边界、进程边界、通信协议、存储实现和 Agent 运行时分开迁移。
- **先契约后拆分：** 先让单体内部通过稳定接口调用，再把本地实现替换为 RPC client。
- **先影子后切流：** 新存储和新服务先接收镜像流量，通过校验后逐步承担读取和写入。
- **数据所有权明确：** Gateway 不访问业务数据库；服务之间不跨库写表；Elasticsearch 和 Redis 不承担消息事实源。
- **事件可演进：** Kafka 事件包含版本、事件 ID、聚合 ID、发生时间和幂等键；消费者兼容至少一个旧版本。
- **兼容旧客户端：** `/messages/offline`、历史 `after_id` 和现有 WS 协议在替代链路完成验收前继续保留。
- **测试驱动：** 每次抽离先补契约测试、故障测试和回滚测试，再改变生产路由。
- **控制服务数量：** 没有压测或团队协作需求时，不单独拆 User、Group、Contact、File 和 Conversation。

## 3. 当前基线

| 领域 | 当前实现 | 演进起点 |
| --- | --- | --- |
| 应用部署 | 三个相同的单体节点 + Nginx | 已具备横向扩展，尚未形成独立服务 |
| HTTP / WS | Gin + WebSocket Hub | 适合收口为 IM Gateway |
| 消息 | `MessageService` + `MessageRepository` | 已有接口、Kafka 和 Outbox，可优先抽离 |
| 同步 | MySQL `user_sync_inbox` + `/sync` | 后端切片已存在，客户端仍使用旧离线接口 |
| 会话 | Kafka 驱动的 Conversation Projection | 可继续留在 Core，后续按压力抽离 |
| 消息存储 | MySQL `messages` | 先抽象 Store，再迁移 Cassandra |
| 事件 | 单节点 Kafka + Transactional Outbox | 需要事件版本、兼容测试和集群化 |
| 实时状态 | Redis Presence / PubSub / Hot Group | 继续保持临时状态职责 |
| 文件 | MinIO | 保持独立对象存储，不随消息库迁移 |
| AI | 进程内 Eino Agent，消费 `message.direct.created` | 已有天然事件边界，第三阶段独立部署 |

## 4. 总体目标架构

```text
Client
  │ WS / HTTP
  ▼
┌──────────────────┐
│   IM Gateway     │  Auth context / rate limit / WS sessions
└───────┬──────────┘
        │ gRPC
        ├──────────────► Dipole Core
        │                User / Group / Contact / File / Auth
        ▼
┌──────────────────┐
│ Message Service  │
└───────┬──────────┘
        │
        ├──────────────► MySQL Cluster
        │                Metadata / idempotency / outbox / conversation state
        │
        └──────────────► Kafka Cluster: message.created.vN
                                  │
          ┌──────────────┬──────────────┬──────────────┬──────────────┐
          ▼              ▼              ▼              ▼              ▼
     Cassandra      Sync Service   Elasticsearch   Realtime       Agent Runtime
     Message Store  Durable Inbox  Search Index    Delivery       Memory/Plan/Tool
     Timeline       / Checkpoint                   Redis+Gateway
```

Redis 继续存储 Presence、连接路由、热点状态、限流和短期缓存。用户 Inbox、设备 Cursor 和消息历史保存在持久化存储中。

## 5. 前置门禁 G0

三阶段实施前先完成以下基线治理，当前仅列入计划：

- [ ] 解决 `AD-001`：改为并发安全的用户级 Sync Sequence，补充提交乱序测试。
- [x] 解决 `AD-002`：消除旧群事件与 `sync_fanout=false` 的协议歧义。
- [x] 解决 `AD-003`：幂等冲突校验消息身份，禁止错误收件人修复 Inbox。
- [ ] 为核心事件建立版本化契约和新旧版本兼容测试。
- [ ] 建立基线压测：发送吞吐、端到端延迟、Kafka lag、Inbox 写放大、热群 fanout。
- [ ] 增加统一 `request_id`、`trace_id`、`event_id`，贯通 HTTP、WS、gRPC、Kafka 和 Outbox。
- [ ] 建立服务级健康检查、指标、结构化日志和最小告警规则。
- [ ] 将需要长期维护的架构 Markdown 纳入版本控制，关闭 `AD-007`。

**G0 验收：** 全量测试通过；事件兼容测试通过；基线压测结果归档；当前单体部署行为保持一致。

### 架构债务治理映射

| 债务 | 计划里程碑 | 解除条件 |
| --- | --- | --- |
| `AD-001` | G0 / A1 | 用户级序号和并发提交乱序测试通过 |
| `AD-002` | G0 / M2 | 新旧群事件兼容测试通过 |
| `AD-003` | G0 / M3 | 幂等身份冲突与收件人隔离测试通过 |
| `AD-004` | A1 / A6 | 热群持久化 checkpoint 和客户端补拉通过 |
| `AD-005` | A2 / A6 | 压测证明写扩散受控，或完成对应投影优化 |
| `AD-006` | M1 | 仓储写入口完成收敛 |
| `AD-007` | G0 | 关键架构文档可随代码审查和追溯 |

## 6. 阶段一：渐进式微服务改造

### M1：收口模块边界与依赖装配

目标是在一个进程内完成边界整理，不增加网络调用。

- [ ] 将 `server.New()` 和 `RegisterKafkaHandlers()` 中的重复构造逻辑收口到 Composition Root。
- [ ] 按领域定义应用端口：`MessageApplication`、`CoreCapability`、`SyncApplication`、`EventPublisher`。
- [ ] 将 repository 接口移动到使用方领域，避免 handler/bootstrap 依赖具体 repository。
- [ ] 禁止跨模块直接 `repository.NewXXXRepository()`，统一通过构造参数注入。
- [ ] 建立架构约束测试，阻止 Gateway/Handler 直接导入数据库实现。
- [ ] 保留 `LocalMessageApplication` 和 `LocalSyncApplication`，确保单体模式继续运行。

**验收：** HTTP/WS 契约不变；`go test ./...`、race 定向测试和现有端到端测试通过；单体镜像仍可独立部署。

### M2：定义远程契约但仍走本地实现

- [ ] 使用 protobuf 定义 Message Command、History Query、Core Authorization 和 Sync Query 契约。
- [ ] 明确错误码、超时、幂等键、分页游标和认证上下文传递规则。
- [ ] 生成 gRPC server/client，并用 in-process adapter 跑同一组契约测试。
- [ ] Kafka Topic 增加 schema version；定义兼容、弃用和死信策略。
- [ ] 增加 `message.transport=local|grpc` 配置开关，默认继续使用 `local`。

**验收：** Local 与 gRPC adapter 通过同一套 contract test；关闭 gRPC 时系统行为与 M1 一致。

### M3：抽离 Message Service

- [ ] 新增 `cmd/message-service`，承接发送、幂等、消息历史、Outbox 和 Message Store 接口。
- [ ] 当前单体先作为 Gateway/Core，通过 gRPC 调用 Message Service。
- [ ] Message Service 通过 Core Capability API 校验用户、好友、群成员和收件人快照，不跨库读取 Core 表。
- [ ] 使用影子请求比对 Local 与 Remote 响应，影子链路禁止产生第二次业务写入。
- [ ] 按节点逐步将 `message.transport` 切换为 `grpc`，保留快速回切能力。
- [ ] 明确 Message Service 数据表所有权，其他进程停止直接写 `messages` 和 `outbox_events`。

**验收：** 发送、历史、文件消息、热群、幂等和 Outbox 故障场景通过；Remote 模式达到基线延迟目标；回切 Local 不需要数据回滚。

### M4：抽离 IM Gateway

- [ ] 新增 `cmd/gateway`，只保留 HTTP/WS、认证上下文、限流、连接管理和协议适配。
- [ ] Gateway 通过 gRPC 调用 Message Service 与 Core，不持有 GORM repository。
- [ ] Kafka Realtime Delivery 将用户事件路由到 Gateway 节点，沿用 Redis Presence。
- [ ] 将静态 Web、Swagger 和管理入口的归属显式化，避免 Gateway 混入后台任务。
- [ ] 保留现有单体入口作为回滚部署，直到 Gateway 完成全流量验证。

**验收：** 多节点 WS 路由、断线重连、踢下线、跨节点投递和滚动升级通过；Gateway 进程断开数据库后仍能正常处理其职责。

### M5：形成最小服务集合

阶段一结束时保持以下部署边界：

```text
dipole-gateway    HTTP / WS / realtime delivery
dipole-core       User / Group / Contact / File / Auth / Conversation
dipole-message    Message command / history / idempotency / outbox
```

Sync 暂时可以随 Message Service 部署，待阶段二具备可重放事件和持久化游标后再独立。User、Group、Contact 和 File 继续留在 Core。

## 7. 阶段二：存储与事件架构重构

### A1：稳定 Timeline 与 Store 抽象

- [ ] 增加会话内单调 `conversation_seq`，消息唯一 ID 与排序序号分离。
- [ ] 增加 `read_seq` 和设备级同步 checkpoint，保留旧 `UnreadCount` 兼容投影。
- [ ] 定义 `MessageStore`、`SyncStore` 和 `SearchIndex` 接口，MySQL 实现先通过完整 contract test。
- [ ] 为热群定义持久化 checkpoint，解决 `AD-004` 后再计划移除旧离线接口。
- [ ] 为消息创建、撤回、编辑和删除预留版本化 mutation 事件，当前只实现已支持的动作。

**验收：** MySQL 实现下的新旧 API 结果一致；Sequence 并发测试、设备同步测试和历史分页测试通过。

### A2：基础设施集群化

- [ ] MySQL Cluster 承载用户、群、联系人、文件元数据、幂等记录、Outbox、Conversation 和迁移控制表。
- [ ] Kafka Cluster 设置明确的 partition key、复制因子、最小 ISR、保留期和 DLQ 监控。
- [ ] Redis 使用可故障转移拓扑，并验证 Presence、PubSub、热点检测和限流语义。
- [ ] Cassandra 与 Elasticsearch 先进入隔离环境，不接生产读流量。
- [ ] Local Compose 保持单节点开发模式，新增 cluster profile 用于集成和故障演练。

**验收：** 单节点故障演练、Kafka 重平衡、MySQL 主节点切换和 Redis 故障转移期间不丢已确认消息。

### A3：Cassandra Message Store 影子投影

- [ ] 设计按 `conversation_id + bucket` 分区、按 `conversation_seq` 聚簇排序的 Timeline 表。
- [ ] 通过 Kafka `message.created.vN` 将 MySQL 已确认消息投影到 Cassandra，消费者按事件 ID 幂等。
- [ ] 先回填历史数据，再持续追平增量；记录 checkpoint 和失败重试。
- [ ] 建立数量、哈希、抽样内容和会话序号连续性校验。
- [ ] Message Service 执行 shadow-read，对比 Cassandra 与 MySQL，客户端仍读取 MySQL。

**验收：** 全量校验达到约定阈值；Kafka lag 可观测；重复消费和乱序事件不会破坏 Timeline。

### A4：渐进切换 Cassandra 读取与写入职责

- [ ] 按用户或会话灰度将历史读取切到 Cassandra，失败时回退 MySQL 并记录差异。
- [ ] 逐步提升 Cassandra 读取比例，持续比较结果和延迟。
- [ ] 稳定后停止向 MySQL 保存完整消息正文，只保留幂等、Outbox、路由和必要元数据。
- [ ] 在停止 MySQL 正文写入前完成备份、回放工具和明确回滚窗口。
- [ ] 达到保留期后再归档或删除 MySQL 历史消息表。

应用层禁止直接同步双写 MySQL 和 Cassandra；跨存储复制通过 Outbox/Kafka 投影完成，避免分布式事务。

**验收：** Cassandra 主读稳定；回退演练通过；数据一致性报告归档；MySQL 压力符合预期下降目标。

### A5：引入 Elasticsearch Search Projection

- [ ] Search Indexer 消费版本化消息事件，按 `message_id` 幂等写入。
- [ ] 使用 index alias 支持重建、切换和回滚；索引映射纳入版本控制。
- [ ] 搜索接口执行会话成员权限校验，索引结果不能绕过 Core 权限。
- [ ] 支持从 Kafka/Message Store 全量重建索引，ES 故障不阻断消息发送。

**验收：** 搜索正确性、权限隔离、重建和 alias 切换测试通过。

### A6：独立 Sync Service 与实时投影

- [ ] 新增 `dipole-sync`，消费消息事件并维护 Durable Inbox、群 checkpoint 和设备 Cursor。
- [ ] 通过 checkpoint、重放和回填保证消费者可恢复，修复事件进入同一幂等模型。
- [ ] 前端增加 IndexedDB/本地游标，先双跑 `/messages/offline` 与 `/sync` 并比较结果。
- [ ] 热群使用 Sync Item 通知客户端按 `conversation_seq` 拉取 Cassandra Timeline。
- [ ] 完成灰度后停止旧接口新增能力，经过一个兼容周期再讨论移除。

**验收：** 离线、多设备、热群、重放、Cursor 恢复和客户端升级测试通过；关闭 Redis 后仍可恢复持久同步状态。

## 8. 阶段三：Agent 化

### G1：抽离现有 Eino Agent Runtime

- [ ] 新增 `dipole-agent`，第一步沿用 Go + Eino，避免同时更换语言和进程边界。
- [ ] 消费版本化 `message.direct.created`，使用独立 consumer group 和幂等执行记录。
- [ ] 删除 Agent 对 GORM repository 的直接依赖，所有读取和动作通过 Capability API。
- [ ] Agent 回复通过 Message Service Command API 发送，禁止直接写消息库。
- [ ] 增加 `agent.mode=embedded|shadow|remote|off`，先 shadow 再 remote。

**验收：** Embedded 与 Remote 对同一测试集产生等价动作；Agent 停机不影响传统 IM；重复事件不会产生重复回复。

### G2：建立受控 Capability API

- [ ] 提供读取会话上下文、用户资料、群信息和消息历史的最小权限接口。
- [ ] 提供发送消息、创建任务等写能力，并为每个 Tool 定义参数 Schema、超时和幂等键。
- [ ] Agent 使用服务身份和用户委托上下文，记录每次 Tool 调用的授权依据。
- [ ] 高风险动作增加审批、撤销或人工确认策略。
- [ ] Capability 层设置调用预算、速率限制和审计日志。

### G3：Agent Runtime 能力化

- [ ] 短期记忆从 Message Service 按会话窗口获取，长期记忆使用独立 Memory Store。
- [ ] 将 Planning、Tool Calling、Memory 和最终 IM Action 分成可追踪步骤。
- [ ] 支持任务状态、超时、取消、重试和失败补偿。
- [ ] 防止 Agent 自己的输出再次触发无限循环，事件中携带 origin/causation 信息。
- [ ] 建立 Prompt、Tool Schema、模型和策略版本管理。

### G4：评估、观测与安全门禁

- [ ] 建立离线评测集：回答质量、工具选择、权限边界、幂等和拒绝行为。
- [ ] 记录模型调用延迟、Token、成本、Tool 成功率和端到端任务成功率。
- [ ] 对 Prompt Injection、越权 Tool、敏感数据外发和循环调用进行专项测试。
- [ ] 模型或策略升级先 shadow evaluation，再按用户灰度。
- [ ] 保留 Agent 总开关，关闭后传统 IM 链路完整可用。

**阶段三验收：** Agent Runtime 可独立部署和扩容；故障与升级不影响 IM Core；权限、审计、成本和效果均可观测。

## 9. 全程测试矩阵

| 测试层 | 必须覆盖 |
| --- | --- |
| Unit | 序号分配、幂等、权限、游标、事件转换、Tool 参数校验 |
| Contract | Local/Remote Service、gRPC、Kafka 新旧 Schema、Store 多实现 |
| Integration | MySQL、Kafka、Redis、Cassandra、Elasticsearch、MinIO |
| End-to-End | HTTP/WS 发送、历史、离线、多端、热群、搜索、Agent Action |
| Migration | 回填、双轨比较、影子读、灰度切换、回滚和重放 |
| Failure | 节点宕机、超时、重复事件、乱序、Kafka lag、存储不可用 |
| Performance | 普通群/热群吞吐、P95/P99 延迟、成员级写放大、搜索和 Agent 延迟 |

每个里程碑都需要更新 `CHANGELOG.md`、本计划状态和 `ARCHITECTURE-DEBT.md`，并保存测试与迁移证据。

## 10. 回滚开关

| 开关 | 值 | 用途 |
| --- | --- | --- |
| `message.transport` | `local / grpc` | Message Service 进程抽离回切 |
| `message.read_store` | `mysql / shadow / cassandra` | Cassandra 读流量灰度 |
| `sync.mode` | `legacy / compare / timeline` | 客户端同步协议迁移 |
| `search.enabled` | `false / true` | ES 故障隔离 |
| `agent.mode` | `off / embedded / shadow / remote` | Agent 抽离与灰度 |

开关只控制路由，不能替代数据回滚方案。每次切换前必须记录数据 checkpoint、兼容窗口和恢复步骤。

## 11. 明确暂不实施

- 暂不把 User、Group、Contact、File、Conversation 拆成五个独立服务。
- 暂不让 Gateway 直接访问 MySQL、Cassandra 或 Elasticsearch。
- 暂不使用 Redis 保存 Durable Inbox、设备 Cursor 或消息事实。
- 暂不在应用事务中直接双写 MySQL 和 Cassandra。
- 暂不同时将 Eino 迁往 Python/TypeScript；独立进程稳定后再依据生态和团队成本评估。
- 暂不移除 `/messages/offline`、`after_id` 和 `UnreadCount` 兼容层。

## 12. 滚动执行顺序

```text
G0 基线门禁
  ↓
M1 模块边界 → M2 远程契约 → M3 Message Service → M4 Gateway
  ↓
A1 Timeline/Store → A2 集群 → A3 Cassandra 影子 → A4 切流
  ↓
A5 Search → A6 Sync Service
  ↓
G1 Agent 抽离 → G2 Capability → G3 Runtime → G4 Eval/Safety
```

任何里程碑未通过验收时停留在当前形态，修复后再进入下一步，避免将未验证风险传递到后续阶段。

## 13. 分支与合并策略

### 主要分支

| 分支 | 覆盖范围 | 创建基线 | 合并条件 |
| --- | --- | --- | --- |
| `epic/01-microservices` | G0、M1-M5 | 最新 `master` | 微服务阶段验收全部通过 |
| `epic/02-storage-architecture` | A1-A6 | 阶段一合并后的 `master` | 存储迁移、回滚和故障测试通过 |
| `epic/03-agent-runtime` | G1-G4 | 阶段二合并后的 `master` | Agent 效果、安全和隔离门禁通过 |

三条 Epic 分支可以提前建立远端引用，用于固定路线。后续阶段开始开发前，必须先合并最新 `master`，确保继承前一阶段的代码、迁移和事件契约。

### 里程碑分支

- 每个里程碑从对应 Epic 分支创建短期分支，例如 `feature/m1-composition-root`、`feature/a3-cassandra-shadow`。
- 一个短期分支只处理一个里程碑或一个可独立回滚的问题，禁止同时跨越微服务、存储和 Agent 三个维度。
- 短期分支完成测试和 diff 审查后合并到 Epic；Epic 达到阶段验收后再合并到 `master`。
- 紧急修复从 `master` 创建 `fix/*`，合并后同步回所有仍活跃的 Epic 分支。
- 禁止对已推送的共享分支执行 force push，避免破坏阶段历史和迁移证据。

### 持续记录

- 每个产生可观察变化的提交同步更新 `CHANGELOG.md` 的 `Unreleased`。
- 架构风险新增、状态变化或关闭时同步更新 `ARCHITECTURE-DEBT.md`。
- 每个里程碑完成后更新本计划复选框、验收证据和关联提交。
- 合并到 `master` 前必须执行对应测试矩阵、`git diff --check` 和敏感信息扫描。
