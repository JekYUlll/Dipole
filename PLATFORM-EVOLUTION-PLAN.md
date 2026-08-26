# Dipole 平台演进计划

> 状态：计划中
>
> 基线：`99f2ef0 feat(sync): add user inbox timeline`
>
> 更新日期：2026-08-26

## 1. 目标

Dipole 按以下顺序完成四次独立演进，并持续维护前端设计轨道：

1. **微服务改造：** 从模块化单体渐进拆出 Gateway、Message 和 Sync 服务，Core 暂时保留 User、Group、Contact、File、Auth。
2. **架构重构：** 建立 MySQL 元数据、Kafka 事件流、Cassandra 消息存储、Elasticsearch 搜索索引和 Redis 实时状态的分层架构。
3. **Agent 化：** 将进程内 Eino AI 模块演进为 TypeScript Agent Runtime，通过事件和受控 Capability API 参与 IM 业务。
4. **C++ 实时数据面：** 在稳定协议和性能基线之上评估 Realtime Delivery 与 Gateway 替换，只在收益可复现时灰度切流。

微服务阶段内先将 GORM 渐进迁移到 sqlc；前端从当前阶段开始维护 Pencil `.pen` 设计基线，并随 IM、Agent 和数据面能力持续更新。

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
- **SQL 是数据契约：** schema 与 query 进入版本控制，sqlc 负责 Go 侧生成；跨语言服务通过 API 和事件协作，不跨边界共享业务表。
- **设计先行：** 用户可见功能先更新 Pencil 设计稿和状态矩阵，再实现 Vue 与视觉回归。

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
| AI | 进程内 Eino Agent，消费 `message.direct.created` | 已有天然事件边界，第三阶段迁移到 TypeScript Runtime |

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

各阶段实施前先完成以下基线治理，当前仅列入计划：

- [x] 解决 `AD-001`：改为并发安全的用户级 Sync Sequence，补充提交乱序测试。
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
| `AD-002` | G0 / M3 | 新旧群事件兼容测试通过 |
| `AD-003` | G0 / M4 | 幂等身份冲突与收件人隔离测试通过 |
| `AD-004` | A1 / A6 | 热群持久化 checkpoint 和客户端补拉通过 |
| `AD-005` | A2 / A6 | 压测证明写扩散受控，或完成对应投影优化 |
| `AD-006` | M1 | 仓储写入口完成收敛 |
| `AD-007` | G0 | 关键架构文档可随代码审查和追溯 |
| `AD-010` | M2 | GORM 和 AutoMigrate 完成退出，sqlc 与 migration 门禁通过 |
| `AD-011` | F1 | canonical `.pen`、设计 token 与现有页面状态完成评审 |

## 6. 阶段一：渐进式微服务改造

### M1：收口模块边界与依赖装配

目标是在一个进程内完成边界整理，不增加网络调用。

- [x] 将 `server.New()` 和 `RegisterKafkaHandlers()` 中的重复 Repository 与消息域 Service 构造收口到 Composition Root。
- [x] 定义 `MessageApplication`、`SyncApplication` 与 `EventPublisher`，并提供 Local adapter。
- [x] 定义 `CoreCapability` 与 Local adapter，供 Message 与 Agent 复用受控的 User/Group/Contact 查询。
- [x] 将 repository 接口保留在使用方 Service，避免 handler 和 transport 依赖具体 repository。
- [x] 禁止跨模块直接 `repository.NewXXXRepository()`，统一由 Composition Root 创建并通过构造参数注入。
- [x] 建立架构约束测试，阻止 Server、Handler 和 Transport 直接导入数据库实现。
- [x] 保留 `LocalMessageApplication` 和 `LocalSyncApplication`，确保单体模式继续运行。

**验收：** HTTP/WS 契约不变；`go test ./...`、race 定向测试和现有端到端测试通过；单体镜像仍可独立部署。

### M2：从 GORM 渐进迁移到 sqlc

- [x] 建立版本化 SQL migration，以空库和现有库升级测试替代运行时 `AutoMigrate`。
- [x] 引入 `database/sql + sqlc`、可复现生成命令、DBTX 事务边界和 domain mapper。
- [x] 对同一 Repository Port 建立迁移契约，按低风险到高风险逐仓储迁移。
- [x] 迁移 Message、Outbox、Sync 事务和 `FOR UPDATE` 锁，并执行真实 MySQL 并发测试。
- [x] 删除 GORM adapter、model tag、SQLite 方言测试和 `gorm.io/*` 依赖。

**验收：** 服务启动不修改 schema；SQL migration、生成漂移、Repository contract、MySQL 集成和回滚测试通过；生产代码不再导入 GORM。

详细步骤见 [GORM 到 sqlc 迁移计划](DATA-ACCESS-MIGRATION.md)。

### M3：定义远程契约但仍走本地实现

- [x] 使用 protobuf 定义 Message Command 与 History Query v1 契约。
- [x] 使用 protobuf 定义 Core Authorization 和 Sync Query 契约，并复用 `common.v1.RequestContext`。
- [x] 明确 Message RPC 错误码、超时、幂等键、分页游标和认证上下文传递规则。
- [x] 生成 Message gRPC server/client，并用 bufconn 验证 Local server 与 Remote client adapters。
- [x] 为 Local 与 gRPC adapters 建立完整共享行为契约，覆盖全部命令和查询。
- [x] Kafka Topic 增加 schema version；定义兼容、弃用和死信策略。
- [x] 增加 `message.transport=local|grpc` 配置开关，默认继续使用 `local`；M3 的 grpc 模式先走 bufconn，M4 再替换为受认证网络 channel。

**验收：** Local 与 gRPC adapter 通过同一套 contract test；关闭 gRPC 时系统行为与 M2 一致。

### M4：抽离 Message Service

- [x] 新增 `cmd/message-service`，承接发送、幂等、消息历史、Outbox 和 Message Store 接口。
- [x] 当前单体先作为 Gateway/Core，通过受认证 gRPC 调用 Message Service，并保留 `local` 回切。
- [x] Message Service 通过 Core Capability API 校验用户、好友、群成员和收件人快照，不跨库读取这些 Core 表。
- [x] 使用异步影子请求比对 Local 与 Remote 查询响应；四类发送命令只执行 primary，影子链路禁止业务写入。
- [x] 提供按节点逐步切换 `message.transport=grpc` 的 shadow/owner 运行模式、consumer group 交接手册和 Local 快速回切能力。
- [x] 明确 Message Service 数据表所有权；远程模式下 Core 停止写 `messages` 和 `outbox_events`，独立进程只组合 Message 与 Outbox adapters。

当前过渡限制：Message 与 Core 仍使用同一 MySQL schema 和数据库账号；代码侧文件所有权已迁入 Core Capability，数据库最小权限继续由 AD-015 跟踪。内部 RPC 已通过 AD-013 完成 TLS 1.3 mTLS 与 caller 身份绑定。

**验收：** 发送、历史、文件消息、热群、幂等和 Outbox 故障场景通过；Remote 模式达到基线延迟目标；回切 Local 不需要数据回滚。

### M5：抽离 IM Gateway

- [x] 新增 `cmd/gateway`，只保留 HTTP/WS、认证上下文、限流、连接管理和协议适配。
- [x] Gateway 通过 gRPC 调用 Message Service 与 Core，不持有数据库 repository。
- [x] Kafka Realtime Delivery 将用户事件路由到 Gateway 节点，沿用 Redis Presence。
- [x] 将静态 Web、Swagger 和管理入口的归属显式化；M5 期间由 Gateway 代理到私网 Core。
- [x] 保留 `gateway.mode=embedded` 单体入口作为无数据回滚部署。

**验收：** 多节点 WS 路由、断线重连、踢下线、跨节点投递和滚动升级通过；Gateway 进程断开数据库后仍能正常处理其职责。

### M6：形成最小服务集合

阶段一结束时保持以下部署边界：

```text
dipole-gateway    HTTP / WS / realtime delivery
dipole-core       User / Group / Contact / File / Auth / Conversation
dipole-message    Message command / history / idempotency / outbox
```

Sync 暂时可以随 Message Service 部署，待阶段二具备可重放事件和持久化游标后再独立。User、Group、Contact 和 File 继续留在 Core。

- [x] 统一镜像打包 Core、Message、Gateway 与 migration 四个二进制，旧单体入口继续作为默认 entrypoint。
- [x] 增加独立微服务 Compose，Core/Message/Gateway 使用 TLS 1.3 mTLS、独立 caller 与健康依赖启动。
- [x] Gateway 不依赖 MySQL service，Core 与 Message 继续使用当前 MySQL schema，表级账号由 AD-015 跟踪。
- [x] 增加可重复 smoke，覆盖 migration、冷启动、Gateway health、Core HTTP 代理和 remote WS 所有权。

**验收：** 隔离 Compose project 全部长期服务 healthy；Gateway 只暴露公开端口；自动 smoke 完成后可无残留销毁拓扑。

## 7. 阶段二：存储与事件架构重构

### A1：稳定 Timeline 与 Store 抽象

- [x] 增加会话内单调 `conversation_seq`，消息唯一 ID 与排序序号分离。
- [x] 增加 `read_seq` 和设备级同步 checkpoint，保留旧 `UnreadCount` 兼容投影。
- [x] 定义 `MessageStore`、`SyncStore` 和 `SearchIndex` 接口，MySQL 实现先通过完整 contract test。
- [x] 为热群定义持久化 checkpoint，解决 `AD-004` 后再计划移除旧离线接口。
- [x] 为消息创建、撤回、编辑和删除预留版本化 mutation 事件，当前只实现已支持的动作。

**验收：** MySQL 实现下的新旧 API 结果一致；Sequence 并发测试、设备同步测试和历史分页测试通过。

### A2：基础设施集群化

- [x] MySQL Cluster 承载用户、群、联系人、文件元数据、幂等记录、Outbox、Conversation 和迁移控制表。
- [x] Kafka Cluster 设置明确的 partition key、复制因子、最小 ISR、保留期和 `acks=all`，并验证单 broker 故障与 quorum 恢复。
- [x] 固定 Kafka consumer rebalance policy，验证成员退出后的 partition 接管与 lag 归零，并提供进程内处理结果 snapshot。
- [x] 增加 lag、under-replicated partitions、retry 和 DLQ 的 Prometheus 监控门禁。
- [x] Redis 使用可故障转移拓扑，并验证 Presence、PubSub、热点检测和限流语义。
- [x] Cassandra 与 Elasticsearch 先进入隔离环境，不接生产读流量。
- [x] Local Compose 保持单节点开发模式，新增 cluster profile 用于集成和故障演练。

**验收：** 单节点故障演练、Kafka 重平衡、MySQL 主节点切换和 Redis 故障转移期间不丢已确认消息。

### A3：Cassandra Message Store 影子投影

- [x] 设计按稳定 `conversation_key + bucket` 分区、按 `conversation_seq` 聚簇排序的 Timeline 表。
- [x] 通过 Kafka `message.created.vN` 将 MySQL 已确认消息投影到 Cassandra，消费者按事件负载幂等。
- [x] 先回填历史数据，再持续追平增量；记录 checkpoint 和失败重试。
- [x] 建立数量、哈希、抽样内容和会话序号连续性校验。
- [x] Message Service 执行 shadow-read，对比 Cassandra 与 MySQL，客户端仍读取 MySQL。

**验收：** 全量校验达到约定阈值；Kafka lag 可观测；重复消费和乱序事件不会破坏 Timeline。

### A4：渐进切换 Cassandra 读取与写入职责

- [x] 按会话灰度将 Direct/Group Seq 历史读取切到 Cassandra，失败时使用同一 Seq cursor 整页回退 MySQL。
- [ ] 逐步提升 Cassandra 读取比例，持续比较结果和延迟。
- [x] 首批按会话稳定 cohort 灰度群 `after_seq` 增量读取，缺页或存储错误自动回退 MySQL，百分比 0 可即时回切。
- [x] 增加 Direct/Group `before_seq` HTTP/RPC 契约，Web 首屏、历史分页与热群补拉统一使用 Seq cursor domain。
- [x] 暴露 Cassandra/MySQL fallback 路由计数和延迟指标，并通过真实双存储缺行演练。
- [x] 增加可配置主读抽样核验、match/mismatch/error 指标；Seq 连续但 payload 被篡改时整页回退 MySQL。
- [x] 增加 fallback ratio、payload mismatch 与 verification dependency 告警，并用 promtool 固定时序验证停止门禁。
- [x] 审计 MySQL 正文依赖，确认 Sync 补全、旧 Offline、UUID/幂等回放、文件授权和迁移校验尚未具备完整替代契约（AD-019）。
- [ ] 持续提升 Cassandra 读取比例并完成生产观测；A4 期间继续保存 MySQL 完整消息，保留对账与即时回切基准。
- [ ] A5/A6 替代读契约双跑通过后，再停止向 MySQL 保存完整消息正文，只保留幂等、Outbox、路由和必要元数据。
- [ ] 在切换为 metadata-only 写入前完成固定快照备份、事件回放演练、责任人和明确回滚窗口。
- [ ] 达到保留期后再归档或删除 MySQL 历史消息表。

应用层禁止直接同步双写 MySQL 和 Cassandra；跨存储复制通过 Outbox/Kafka 投影完成，避免分布式事务。

**验收：** Cassandra 主读稳定；回退演练通过；数据一致性报告归档；MySQL 压力符合预期下降目标。

### A5：引入 Elasticsearch Search Projection

- [x] 固化 `dipole-messages-v1` strict mapping、read/write Alias、schema readiness 和原子 Alias 切换契约。
- [x] 实现 storage-neutral Elasticsearch Search adapter，以 Message UUID 为 `_id`、external revision 与 payload hash 分类重复、乱序和冲突事件。
- [x] 独立 Search Indexer 使用专属 Kafka consumer group 消费八类版本化 mutation，以 `message_id` 和 revision 幂等投影。
- [x] 解决 AD-020，以版本化 tombstone 处理 recall/delete，并让 MySQL/Elasticsearch 共享 mutation contract。
- [x] 实现 Backfill/Reconcile 和 Alias 运维命令，完成真实重建、切换和回滚演练。
- [x] 搜索接口执行会话成员权限校验，索引结果不能绕过 Core 权限。
- [x] 使用固定 Outbox mutation 高水位全量重建索引，ES 故障不阻断消息发送。
- [x] 完成 Pencil Search desktop/mobile 的 Results、Loading、Empty、Error 四态和可复用组件。
- [x] 实现默认关闭的 Vue Search 工作区、Gateway/前端双开关、请求防抖、乱序响应淘汰和组件测试。
- [ ] 搜索全量重建源切到 Cassandra 或归档事件，解除对 MySQL 历史正文的恢复依赖。

**验收：** 搜索正确性、权限隔离、重建和 alias 切换测试通过。

### A6：独立 Sync Service 与实时投影

- [x] 建立独立 `dipole-sync` 查询/Checkpoint 运行时、sqlc 仓储边界和最小权限内部 RPC；Message 事务暂时继续原子写 Inbox。
- [x] 增加默认 Local 的 Core `sync.transport` 切流开关，独立服务不可用时可无数据迁移地回切进程内实现。
- [x] 增加只读 Sync 影子比较，覆盖 Inbox 页面、设备 Cursor 和群 checkpoint，禁止影子推进 Cursor。
- [x] 为 `dipole-sync` 增加默认关闭的独立 Kafka consumer，按事件时收件人快照维护 Durable Inbox；精确重放幂等，冲突整批回滚，热群跳过用户 fanout。
- [ ] 将群 checkpoint/设备 Cursor 的恢复事件纳入独立投影。
- [x] 让 Sync 新 consumer group 从 earliest retained offset 追平，完成 backlog、consumer lag、retry/DLQ 告警与故障演练。
- [x] 增加固定 Outbox 高水位、lease/checkpoint Replay 和 recipient/locator Reconcile；差异报告返回退出码 2，修复事件复用在线投影幂等模型。
- [x] 审计 created Outbox 历史覆盖，为缺少 created Outbox 的 Inbox 建立固定高水位、SHA-256 不可变 baseline、精确 Reconcile 与保序 Restore，解决 `AD-024`。
- [x] 验证 earliest consumer 与固定 Outbox Replay 拼接后的在线追平窗口，并以 lag=0、retry/DLQ 无增量和 Reconcile 一致作为停止门槛。
- [x] 迁移 Message 的 Inbox 写责任和数据库权限：Sync/Message 使用操作级最小账号，`projector` 停止 Message Inbox 写入，`atomic` 保留一键恢复窗口，并通过真实 MySQL 演练解决 `AD-023`。
- [x] 前端增加默认关闭的 IndexedDB Sync Engine，以同一事务提交消息和本地游标，恢复/重连后再显式 ACK 服务端设备 Cursor。
- [x] 增加 `shadow` 双跑模式、持久化 UUID 基线/pending 窗口和 Prometheus 聚合遥测；首批只比较两个协议语义一致的收到私聊消息。
- [ ] 完成真实客户端观察窗口：match 样本达到门槛，grace 后 `legacy_only/sync_only/overflow` 持续为零，再结束旧 Offline 兼容窗口。
- [ ] 默认启用前统一显式退出、401、WS kick 与账号切换的本地清理，并建立容量水位和安全淘汰策略，解决 `AD-025`。
- [ ] 热群使用 Sync Item 通知客户端按 `conversation_seq` 拉取 Cassandra Timeline。
- [x] Sync Item 固化 `conversation_key + message_seq + message_uuid` 定位契约并通过 HTTP/gRPC 暴露。
- [x] 建立 storage-neutral Message hydrator；Sync 返回继续取自 MySQL，并按 locator 异步比较 Cassandra Timeline，覆盖 match、payload mismatch、缺失投影和依赖错误且不影响主响应。
- [ ] 达到观察门槛后为 Cassandra hydration 增加受控主读与 MySQL fallback；切换前补齐告警、灰度比例和无 MySQL 内部 ID 的兼容审计。
- [ ] 为重复发送返回值和文件消息授权建立独立持久元数据契约，停止依赖 MySQL 完整正文。
- [ ] 完成灰度后停止旧接口新增能力，经过一个兼容周期再讨论移除。

**验收：** 离线、多设备、热群、重放、Cursor 恢复和客户端升级测试通过；关闭 Redis 后仍可恢复持久同步状态。

## 8. 阶段三：Agent 化

### G1：固化迁移基线与 Capability API

- [ ] 将现有 Go/Eino Agent 固化为行为基线，建立事件、回复、Tool 轨迹和权限评测集。
- [ ] 删除 Agent 对数据库 repository 的直接依赖，读取和动作统一进入版本化 Capability API。
- [ ] 引入由认证系统生成的 `ExecutionContext`，模型不能设置 principal、tenant、权限和审计身份。
- [ ] Agent 回复通过 Message Service Command API 发送，禁止直接写消息库。
- [ ] 增加 `agent.mode=embedded|shadow|remote|off`，保留 Eino 回滚窗口。

**验收：** Capability contract test 通过；Embedded 基线可重复评测；Agent 停机不影响传统 IM。

### G2：建立 TypeScript Agent Runtime

- [ ] 新增 `dipole-agent`：TypeScript、Node.js、Fastify、Vercel AI SDK、Zod 和 Kafka。
- [ ] 实现 `ExecutionContext`、Capability Registry、Policy Engine、模型路由与调用预算。
- [ ] 使用独立 consumer group 消费版本化事件，通过事件 ID 和 Task ID 幂等。
- [ ] 首先运行 shadow consumer，只记录计划、Tool 轨迹和结果，不执行写操作。
- [ ] Runtime 核心保持框架中立，Mastra、OpenAI Agents SDK 和 LangGraph.js 仅作为参考或 adapter。

### G3：Durable Task、Context 与 Memory

- [ ] 使用 Temporal TypeScript SDK 实现 AgentTask 状态机、Signal、Timer、Retry、取消和恢复。
- [ ] 实现 Context Compiler，按预算组合策略、任务、会话证据、检索、Memory 和 Tool Schema。
- [ ] 引入 Working、Episodic、Semantic、Procedural 和 Observational Memory，并记录来源与作用域。
- [ ] 实现 Event Subscription 与低成本预筛选，相关事件才创建高成本 Agent Task。
- [ ] 支持 `WAITING_INPUT`、`WAITING_APPROVAL` 和版本化 Artifact。
- [ ] 事件携带 origin、causation 和 task 标识，阻断同一因果链的循环触发。

### G4：MCP、评估、观测与安全门禁

- [ ] Runtime 作为 MCP Client 接入外部工具，并以 MCP Server 暴露受控 Dipole Capability。
- [ ] 建立 outcome、trajectory、permission、retrieval 和 cost 五类离线评测。
- [ ] 通过 OpenTelemetry 记录 Task、Run、ContextCompile、ModelCall、ToolCall、Approval 和 Artifact span。
- [ ] 对 Prompt Injection、越权 Tool、敏感数据外发、重复事件和循环调用进行专项测试。
- [ ] 模型、Prompt、Tool Schema 与 Memory Policy 升级先离线评测，再 shadow，最后按用户灰度。
- [ ] 保留 Agent 总开关；A2A、多 Agent 与 MCP experimental Tasks 在核心门禁通过后评估。

**阶段三验收：** Agent Runtime 可独立部署和扩容；故障与升级不影响 IM Core；权限、审计、成本和效果均可观测。

详细设计见 [Agent Runtime 设计](AGENT-RUNTIME-DESIGN.md)。

## 9. 阶段四：C++ 实时数据面

该阶段只在微服务与现代存储架构稳定后启动，并与 Agent Runtime 保持独立发布。优先评估 Realtime Delivery，再依据数据决定是否替换 Gateway。

### C1：建立 Go 数据面基准

- [ ] 固化连接数、消息吞吐、P50/P95/P99、CPU、RSS、context switch 和故障恢复基线。
- [ ] 将连接管理、投递路由、背压、重试和热群聚合定义为版本化协议与 contract test。
- [ ] 明确 Gateway 与 Delivery 的进程边界，禁止 C++ 数据面访问业务数据库。

### C2：C++ Realtime Delivery Shadow

- [ ] 实现 Kafka 消费、Redis Presence 查询、节点级批处理、有界队列、背压和 QoS。
- [ ] 与 Go Delivery 并行消费 shadow 流量，比较目标节点、收件人、顺序和延迟，不重复投递客户端。
- [ ] 通过压测与故障注入证明收益；收益不足时保留 Go 实现并停止替换。

### C3：灰度切换与 Gateway 评估

- [ ] 按节点或用户灰度将投递切到 C++，保留 Go 回切开关和独立 consumer group。
- [ ] 完成 crash isolation、重平衡、Redis 故障、慢消费者和队列溢出演练。
- [ ] Delivery 稳定后再评估 C++ WebSocket Gateway；cgo 仅用于接口窄、批处理明确的 native codec 实验。

**阶段四验收：** C++ 实现通过同一 contract test，在目标负载下取得可复现收益，故障不会影响 Go 业务控制面，并完成自动回切演练。

## 10. 持续轨道：Pencil 前端设计

- [ ] F1：已建立 `design/dipole-ui.pen`、首组 design tokens 与 Search 核心组件；Login/Chat desktop/mobile 待完成。
- [ ] F2：Search 四态及 Vue 工作区已完成；Sync 状态矩阵、desktop/mobile 恢复稿和标题栏状态已完成，Contact、Group、File、Device 与 Settings 待完成。
- [ ] F3：覆盖 Agent Definition、Subscription、Task、Approval、Elicitation、Memory 与 Artifact。
- [ ] F4：已建立 Pencil 增量更新、设计日志和 Vitest 组件测试基线；Vue token 映射、Playwright E2E 与视觉回归待完成。

设计轨道不阻塞后端内部重构；任何用户可见功能进入实现前，必须先完成对应 `.pen` frame 和状态评审。详细步骤见 [Pencil 前端设计计划](FRONTEND-DESIGN-PLAN.md)。

## 11. 全程测试矩阵

| 测试层 | 必须覆盖 |
| --- | --- |
| Unit | 序号分配、幂等、权限、游标、事件转换、Tool 参数校验 |
| Contract | Local/Remote Service、gRPC、Kafka 新旧 Schema、Store 多实现 |
| Integration | MySQL、Kafka、Redis、Cassandra、Elasticsearch、MinIO |
| End-to-End | HTTP/WS 发送、历史、离线、多端、热群、搜索、Agent Action |
| Migration | 回填、双轨比较、影子读、灰度切换、回滚和重放 |
| Failure | 节点宕机、超时、重复事件、乱序、Kafka lag、存储不可用 |
| Performance | 普通群/热群吞吐、P95/P99 延迟、成员级写放大、搜索和 Agent 延迟 |
| Data access | SQL migration、sqlc 生成漂移、Repository contract、真实 MySQL 事务 |
| Frontend | Vue 类型检查、组件、Playwright E2E、视觉回归、响应式和可访问性 |

每个里程碑都需要更新 `CHANGELOG.md`、本计划状态和 `ARCHITECTURE-DEBT.md`，并保存测试与迁移证据。

## 12. 回滚开关

| 开关 | 值 | 用途 |
| --- | --- | --- |
| `message.transport` | `local / grpc` | Message Service 进程抽离回切 |
| `message.read_store` | `mysql / shadow / cassandra` | Cassandra 读流量灰度 |
| `message.mysql_write_mode` | `full / metadata_only` | A5/A6 门禁完成后的 MySQL 正文退役；初始固定为 `full` |
| `message.inbox_write_mode` | `atomic / projector` | Inbox 写责任迁移；`atomic` 是默认回滚路径 |
| `sync.mode` | `legacy / compare / timeline` | 客户端同步协议迁移 |
| `search.enabled` | `false / true` | ES 故障隔离 |
| `agent.mode` | `off / embedded / shadow / remote` | Agent 抽离与灰度 |
| `realtime.delivery` | `go / shadow / cpp` | C++ Delivery 影子验证与回切 |

开关只控制路由，不能替代数据回滚方案。每次切换前必须记录数据 checkpoint、兼容窗口和恢复步骤。

## 13. 明确暂不实施

- 暂不把 User、Group、Contact、File、Conversation 拆成五个独立服务。
- 暂不让 Gateway 直接访问 MySQL、Cassandra 或 Elasticsearch。
- 暂不使用 Redis 保存 Durable Inbox、设备 Cursor 或消息事实。
- 暂不在应用事务中直接双写 MySQL 和 Cassandra。
- 暂不在 AD-019 的 Sync、幂等、文件授权、备份和回放门禁完成前启用 MySQL `metadata_only` 写入。
- 暂不在 Capability 契约与 Eino 基线评测完成前让 TypeScript Runtime 执行生产写操作。
- 暂不因技术栈覆盖直接替换 Go Gateway；C++ 替换必须先通过独立基准和 shadow 对比。
- 暂不移除 `/messages/offline`、`after_id` 和 `UnreadCount` 兼容层。
- 暂不让 TypeScript 或 C++ 服务直接复用 Go 的 sqlc 生成代码或跨服务访问 Core 数据表。
- 暂不在设计稿缺少 mobile、loading、empty、error 和 offline 状态时直接重写对应页面。

## 14. 滚动执行顺序

```text
G0 基线门禁
  ↓
M1 模块边界 → M2 sqlc → M3 远程契约 → M4 Message Service → M5 Gateway → M6 最小服务集
  ↓
A1 Timeline/Store → A2 集群 → A3 Cassandra 影子 → A4 切流
  ↓
A5 Search → A6 Sync Service
  ↓
  ├─ G1 Capability → G2 TS Runtime → G3 Durable Agent → G4 Eval/Safety
  └─ C1 Benchmark → C2 C++ Delivery Shadow → C3 Gray Release
```

任何里程碑未通过验收时停留在当前形态，修复后再进入下一步，避免将未验证风险传递到后续阶段。

Agent 与 C++ 在 A6 之后可以并行推进，但不得在同一里程碑分支中修改相同运行链路。C++ Gateway 评估必须等待 Delivery 灰度稳定。

前端 F1 可以在 M1 期间开始；F2 随现代 IM API 推进，F3 随 Agent 状态机推进。设计资产与实现按独立短分支交付。

## 15. 分支与合并策略

### 主要分支

| 分支 | 覆盖范围 | 创建基线 | 合并条件 |
| --- | --- | --- | --- |
| `epic/01-microservices` | G0、M1-M6，含 sqlc 迁移 | 最新 `master` | 微服务与数据访问阶段验收全部通过 |
| `epic/02-storage-architecture` | A1-A6 | 阶段一合并后的 `master` | 存储迁移、回滚和故障测试通过 |
| `epic/03-agent-runtime` | G1-G4 | 阶段二合并后的 `master` | Agent 效果、安全和隔离门禁通过 |
| `epic/04-cpp-realtime` | C1-C3 | 阶段二合并后的 `master` | 性能收益、故障隔离和回切门禁通过 |
| `epic/05-frontend-experience` | F1-F4 | 最新 `master` | 设计、交互、视觉和可访问性门禁通过 |

五条 Epic 分支可以提前建立远端引用，用于固定路线。后续阶段开始开发前，必须先合并最新 `master`，确保继承前一阶段的代码、迁移和事件契约。

### 里程碑分支

- 每个里程碑从对应 Epic 分支创建短期分支，例如 `feature/m1-composition-root`、`feature/a3-cassandra-shadow`。
- 一个短期分支只处理一个里程碑或一个可独立回滚的问题，禁止同时跨越微服务、存储、Agent 和 C++ 数据面多个维度。
- 短期分支完成测试和 diff 审查后合并到 Epic；Epic 达到阶段验收后再合并到 `master`。
- 紧急修复从 `master` 创建 `fix/*`，合并后同步回所有仍活跃的 Epic 分支。
- 禁止对已推送的共享分支执行 force push，避免破坏阶段历史和迁移证据。

### 持续记录

- 每个产生可观察变化的提交同步更新 `CHANGELOG.md` 的 `Unreleased`。
- 架构风险新增、状态变化或关闭时同步更新 `ARCHITECTURE-DEBT.md`。
- 每个里程碑完成后更新本计划复选框、验收证据和关联提交。
- 合并到 `master` 前必须执行对应测试矩阵、`git diff --check` 和敏感信息扫描。
