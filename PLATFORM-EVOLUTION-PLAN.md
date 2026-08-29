# Dipole 平台演进计划

> 状态：计划中
>
> 基线：`7a209ae merge: record eino agentic capability assessment`
>
> 更新日期：2026-08-29

## 1. 目标

Dipole 按以下顺序完成四次独立演进，并持续维护前端设计轨道：

1. **微服务改造：** 从模块化单体渐进拆出 Gateway、Message 和 Sync 服务，Core 暂时保留 User、Group、Contact、File、Auth。
2. **架构重构：** 建立 MySQL 元数据、Kafka 事件流、Cassandra 消息存储、Elasticsearch 搜索索引和 Redis 实时状态的分层架构。
3. **Agent 化：** 将进程内 Eino AI 模块演进为 TypeScript Agent Runtime，通过事件和受控 Capability API 参与 IM 业务。
4. **C++ 实时数据面：** 在稳定协议和性能基线之上评估 Realtime Delivery 与 Gateway 替换，只在收益可复现时灰度切流。

微服务阶段内先将 GORM 渐进迁移到 sqlc；前端从当前阶段开始维护 Pencil `.pen` 设计基线，并随 IM、Agent 和数据面能力持续更新。

当前微服务 Go 全量测试已在干净 worktree 中通过：测试显式绑定版本化 `configs/config.dist.yaml`，不改变生产配置搜索路径。

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
- [x] 为全部 Kafka managed topics 建立语言中立 v1 JSON Schema、统一领域 decoder、producer drift 与新旧版本兼容测试；新增受管主题必须先通过契约覆盖门禁。
- [x] 建立基线压测：发送吞吐、端到端延迟、Kafka lag、Inbox 写放大、热群 fanout；标准化报告归档于 `benchmarks/g0-2026-08-27/`。
- [x] 补齐 Conversation State 成功 upsert 的三节点计数与 baseline v2，以 20/100 人普通/热群矩阵量化 AD-005，并保留未归因的后续优化门槛。
- [x] 增加 Conversation projection Repository Histogram、逐节点快照差分与 baseline v3，在 20/100 人普通/热群矩阵中完成 AD-005 的 SQL 边界归因，保留 1000 人和候选行为优化门槛。
- [x] 增加可选 sqlc 批量群 Conversation upsert，保持旧逐成员路径兼容；真实 MySQL contract 与 1000 人 SQL 层性能对照已归档于 `benchmarks/ad005-conversation-batch-2026-08-29/`，端到端 P95 和多轮容量复测仍由 AD-005 跟踪。
- [x] 增加统一 `request_id`、`trace_id`、`event_id`，贯通 HTTP、WS、gRPC、Kafka 和 Outbox。
- [x] 建立服务级健康检查、指标、结构化日志和最小告警规则。
- [x] 将需要长期维护的架构 Markdown 纳入版本控制，以 manifest 和检查脚本持续约束并关闭 `AD-007`。

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

当前部署仍共享 MySQL schema，凭据与表操作已通过 AD-015 分离：Message 使用 `message.mysql.*` 和 atomic/projector 最小账号，文件所有权通过 Core Capability，内部 RPC 通过 AD-013 完成 TLS 1.3 mTLS 与 caller 身份绑定。

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
- [x] 增加不可变完整消息归档与 source-bound Cassandra Job；按固定 MinIO 对象版本恢复后删除 MySQL 正文，仍可完成 Timeline 重建、全量对账和篡改检测。
- [x] 固化重复消息 Cassandra hydration 的 24 小时观测门禁：至少 100 次 hit、零 fallback、零历史无 Seq，并以 promtool 固定时序验证晋级和停止条件。
- [ ] 持续提升 Cassandra 读取比例并完成生产观测；A4 期间继续保存 MySQL 完整消息，保留对账与即时回切基准。
-  - [x] 增加 Cassandra read rollout evidence v1 与低敏 Go CLI：聚合记录 read share、fallback、verification、p95 和 deployment revision，按策略输出 `eligible|blocked`；真实共享环境采集、责任人批准和回切窗口仍待完成。
- [x] 重新执行隔离 Cassandra read-routing smoke，确认 Cassandra 页面读取、payload 损坏和缺失行均按同一 Seq cursor 回退 MySQL；该证据不改变主读比例和生产开关。
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
- [x] 通过隔离 storage-lab 验收 Cassandra 5.0.9、Elasticsearch 9.5.2 和 MinIO CRUD；测试编排的磁盘水位覆盖仅服务受限实验主机，生产配置与默认拓扑保持不变。
- [x] 完成 Pencil Search desktop/mobile 的 Results、Loading、Empty、Error 四态和可复用组件。
- [x] 实现默认关闭的 Vue Search 工作区、Gateway/前端双开关、请求防抖、乱序响应淘汰和组件测试。
- [x] 搜索全量重建支持 MinIO 不可变事件归档源；receipt 固定 object version ID 和 Governance retention，Backfill、Reconcile 与 Alias 共同校验 snapshot ID、高水位和 SHA-256，删除本地副本与历史 Message Outbox 后仍可恢复、重建和回滚。
- [x] 解决 AD-021：以专用最小权限账号执行 receipt/Reconcile/Job 三重绑定的 Outbox dry-run 与分批清理，记录责任人和对象版本；清理后仅凭归档完成空索引重建、对账与 Alias 回滚。

**验收：** 搜索正确性、权限隔离、重建和 alias 切换测试通过。

### A6：独立 Sync Service 与实时投影

- [x] 建立独立 `dipole-sync` 查询/Checkpoint 运行时、sqlc 仓储边界和最小权限内部 RPC；Message 事务暂时继续原子写 Inbox。
- [x] 增加默认 Local 的 Core `sync.transport` 切流开关，独立服务不可用时可无数据迁移地回切进程内实现。
- [x] 增加只读 Sync 影子比较，覆盖 Inbox 页面、设备 Cursor 和群 checkpoint，禁止影子推进 Cursor。
- [x] 为 `dipole-sync` 增加默认关闭的独立 Kafka consumer，按事件时收件人快照维护 Durable Inbox；精确重放幂等，冲突整批回滚，热群跳过用户 fanout。
- [x] 固化 checkpoint 恢复边界：Inbox 与群 Timeline 高水位可由事件重建；设备 Cursor 与群 `pulled_message_seq` 只能由已持久化客户端 ACK 单调推进，禁止从消息事件推导。
- [x] 让 Sync 新 consumer group 从 earliest retained offset 追平，完成 backlog、consumer lag、retry/DLQ 告警与故障演练。
- [x] 增加固定 Outbox 高水位、lease/checkpoint Replay 和 recipient/locator Reconcile；差异报告返回退出码 2，修复事件复用在线投影幂等模型。
- [x] 审计 created Outbox 历史覆盖，为缺少 created Outbox 的 Inbox 建立固定高水位、SHA-256 不可变 baseline、精确 Reconcile 与保序 Restore，解决 `AD-024`。
- [x] 验证 earliest consumer 与固定 Outbox Replay 拼接后的在线追平窗口，并以 lag=0、retry/DLQ 无增量和 Reconcile 一致作为停止门槛。
- [x] 迁移 Message 的 Inbox 写责任和数据库权限：Sync/Message 使用操作级最小账号，`projector` 停止 Message Inbox 写入，`atomic` 保留一键恢复窗口，并通过真实 MySQL 演练解决 `AD-023`。
- [x] 前端增加默认关闭的 IndexedDB Sync Engine，以同一事务提交消息和本地游标，恢复/重连后再显式 ACK 服务端设备 Cursor。
- [x] 增加 `shadow` 双跑模式、持久化 UUID 基线/pending 窗口和 Prometheus 聚合遥测；首批只比较两个协议语义一致的收到私聊消息。
- [x] 固化 24 小时 Web Sync 观测门禁：至少 100 个 match、零终态单边差异、零 overflow，并以 promtool 固定时序验证晋级和停止条件。
- [ ] 使用候选 commit/bundle 哈希绑定的 Observation Session/Evidence 完成真实客户端观察窗口：match 样本达到门槛，grace 后 `legacy_only/sync_only/overflow` 持续为零，并归档 Prometheus 原始响应和对象版本后，再结束旧 Offline 兼容窗口。
- [x] 统一显式退出、HTTP 401、WS kick 与账号切换的 Session Termination；凭据先撤销，IndexedDB 清理等待在途同步收敛，快速重登等待旧清理完成。
- [x] 建立 IndexedDB 高低容量水位、按会话保底的最近消息安全淘汰、缓存 manifest 和 quota error 状态；淘汰与 Cursor 提交保持同一事务且不额外推进安全游标。
- [x] 建立 Playwright 三浏览器 IndexedDB 验收，覆盖淘汰、重开、账号隔离、延迟清理和页面中断事务原子性；增加 `storage_full/sync_error` 聚合指标与 promtool 告警。
- [x] 使用独立 Chromium persistent profile 在 `commitPage` pending 窗口触发完整浏览器主进程 crash；同一 profile 重启后 Message、manifest 与安全 Cursor 保持整页原子性。
- [x] 使用无特权 user/mount namespace 和 128 MiB tmpfs 触发真实 Chromium IndexedDB 容量拒绝；释放 reserve 后验证失败页不推进安全 Cursor，现有 `storage_full` 分类有效。
- [x] 完成真实浏览器配额、共享设备 HTTP 401/WS kick 和完整进程强退验收，关闭 `AD-025`。
- [x] Web Sync Engine 将热群补拉消息与群 `message_seq` 原子写入 IndexedDB，落库后再 ACK 设备群 checkpoint；`off` 模式保持不 ACK 的内存兼容路径。
- [x] 补齐 Direct Timeline `after_seq` 的 HTTP、Message v1 gRPC、Local/Remote/Shadow 与 Cassandra cohort/fallback 契约，使单聊和群聊共享会话 Seq 增量语义。
- [x] 增加默认关闭的 `sync.item.notify.v1` shadow 协议；通知只携带版本化 locator，现有完整 WS 正文继续投递，热群保留单一聚合 notify + pull 路径。
- [x] 打通 Gateway/WS `message.timeline_notify_mode=primary` 与 Web `VITE_TIMELINE_NOTIFY_MODE=primary` 配置契约；primary 仍只投递无正文 locator，客户端完成连续序列和 UUID 校验后补拉，服务端 Cassandra 观测门禁独立控制。
- [x] 增加 Web Timeline shadow verifier、会话级补洞/去重和有界遥测；固化完整 24 小时、至少 100 次 match、零 missing/mismatch/error/invalid 的晋级门禁。
- [x] 增加 Web Timeline notify primary 客户端路径：按通知的 `conversation_seq` 串行补拉，完成 UUID/序列连续性校验后才交付消息；`off|shadow` 保持兼容，服务端 Cassandra 主读灰度证据仍未晋级。
- [ ] 在线 Sync Item 通知直接驱动客户端按 `conversation_seq` 拉取 Cassandra 主 Timeline，并完成主读灰度门禁。
- [x] Sync Item 固化 `conversation_key + message_seq + message_uuid` 定位契约并通过 HTTP/gRPC 暴露。
- [x] 建立 storage-neutral Message hydrator；Sync 返回继续取自 MySQL，并按 locator 异步比较 Cassandra Timeline，覆盖 match、payload mismatch、缺失投影和依赖错误且不影响主响应。
- [ ] 达到观察门槛后为 Cassandra hydration 增加受控主读与 MySQL fallback；切换前补齐告警、灰度比例和无 MySQL 内部 ID 的兼容审计。
  - [x] 增加 Sync Cassandra hydration evidence v1 与低敏 Go CLI，统一 shadow/primary 的命中、fallback、缺失/冲突/错误和 p95 门禁；真实客户端窗口、责任人批准和生产主读仍待完成。
- [x] 增加 Prometheus snapshot adapter 与 `sync-cassandra-hydration-snapshot` CLI，将运行时低敏指标转换为既有 evidence v1；真实共享环境归档、missing/conflict 端到端归因、责任人批准和自动回切仍待完成。
- [x] 将 snapshot 转换改为起止快照差分，拒绝 counter reset 与 histogram 桶漂移，确保 evidence 计数对应明确窗口；真实共享环境采集与责任人批准仍待完成。
- [x] 为 snapshot parser 增加 metric family、类型、标签唯一性和 histogram 单调性校验；真实共享环境采集、责任人批准与可执行回切仍待完成。
- [x] 增加默认关闭的 `sync.cassandra_primary_hydration`，以 locator 为边界优先读取 Cassandra，失败立即回退 MySQL，并拒绝与 shadow hydration 同时启用；真实灰度和停止门禁仍待完成。
- [x] migration v12 建立 Message Metadata v1，消息事务原子保存幂等 locator、会话 Seq、文件绑定、过期时间和 payload hash；文件授权已停止查询完整消息正文。
- [x] 增加默认关闭的 Cassandra 幂等响应 hydration：Metadata 校验后按会话 Seq 精确读取 Timeline，缺失/冲突回退 MySQL，并以有界指标记录切换证据。
- [x] 为 Sync Cassandra primary/fallback hydration 接入低基数运行时计数与耗时 collector，保留原有日志观测和 MySQL 即时回退；真实客户端窗口、共享环境采集、责任人批准与可执行回切仍待完成。
- [x] 修正 migration 集成测试的最高版本基线至 v47，并通过真实 MySQL/Cassandra smoke 验证 Metadata 回填、hydration 和重复消息恢复。
- [ ] 将重复发送完整返回从 Metadata locator + MySQL Message 回读切换为 Metadata locator + Cassandra hydration，解除最后的正文依赖。
- [x] 修正 migration integration baseline 至 v49，并在隔离 Cassandra/MySQL smoke 中验证 Metadata backfill、重复响应 hydration 和 Legacy ID 恢复；共享环境主读灰度仍需独立证据。
- [ ] 完成灰度后停止旧接口新增能力，经过一个兼容周期再讨论移除。

**验收：** 离线、多设备、热群、重放、Cursor 恢复和客户端升级测试通过；关闭 Redis 后仍可恢复持久同步状态。

## 8. 阶段三：Agent 化

### G1：固化迁移基线与 Capability API

- [x] 将现有 Go/Eino Agent 固化为行为基线，以语言中立 v1 数据集覆盖事件、回复、Tool 轨迹和权限评测。
- [x] 引入由认证触发链生成的 `ExecutionContext`，模型不能设置 principal、Agent 身份和关联审计 ID，所有 Embedded Tool 缺失上下文时 fail closed。
- [x] 删除 Agent ContextBuilder/Tool 对数据库 repository-shaped port 的直接依赖，读取和动作统一进入 `dipole.agent.capability.v1`；本地 adapter 复用 Core、Conversation 与 Message application 边界。
- [x] 在 Capability API 中补充 `AgentPolicyV1`：tenant、委托身份、细粒度 permission、`read|write|destructive` 风险和 approval 语义，并在 Tool/adapter 双层授权。
- [x] 将 permission grant 与 approval 持久化到版本化 Agent Definition 和 AgentTask，支持 scope、过期、撤销及 arguments hash 重校验（`AD-027`）。
  - [x] migration v16 与 `AgentPolicyStoreV1` 已持久化不可变 Definition grant、固定版本 Task、scope/arguments/nonce 绑定 Approval，并以条件更新保证一次性消费。
  - [x] Embedded trigger 从持久 Task policy snapshot 解析 Invocation；`ai.policy_mode=persistent` 默认启用，`static` 保留显式回滚，Tool/Capability/Command 按 resource scope fail closed。
  - [x] migration v17 将 Agent policy 身份列 expand-only 扩至 24 字符，覆盖默认 21 字符 Assistant UUID；真实 MySQL 8.4 验证 Definition 初始化、Task 固定版本和完成迁移。
- [x] Agent 回复通过版本化 `dipole.agent.command.v1` 进入 Message Service：可信 Invocation 固定 sender/target，稳定 Command ID 映射到 Message 幂等键并保留 correlation；普通回复和系统 Tool 均不直接写消息库。
  - [x] 增加 sender-scoped Message Command receipt：复用 sqlc sender/client key 返回 `absent|committed`，Agent 在独立有界恢复窗口内核对完整消息绑定，收敛远程超时的不确定结果。
  - [x] 增加绑定 running Tool Invocation 的认证 MCP Command RPC：Core 派生 Command ID 与身份、复算 canonical Tool 参数摘要并返回 Message action reference；生产 write Tool 继续关闭。
- [x] 增加 `ai.runtime_mode=off|embedded|shadow|remote`，兼容旧 `ai.enabled`；shadow 保留 Go 权威写入，remote 停止注册 Embedded consumer，为 Eino 回滚和 TS 切流建立开关。

**验收：** Capability contract test 通过；Embedded 基线可重复评测；Agent 停机不影响传统 IM。

### G2：建立 TypeScript Agent Runtime

- [x] 新增 `dipole-agent`：TypeScript、Node.js、Fastify、Vercel AI SDK、Zod 和 Kafka。
  - [x] 建立 `agent-runtime/` Node 22+ foundation：Fastify 健康面、Zod trusted ExecutionContext、Go 兼容 Task ID、Capability Registry、resource-scope Policy Engine 与只读 shadow processor。
  - [x] 增加 KafkaJS adapter、兼容 v1 Message decoder、独立 `dipole-agent-shadow-*` group、冷启动有界重连与 Compose 服务。
- [x] 实现 `ExecutionContext`、Capability Registry、Policy Engine、provider-neutral 模型路由与每 Run 调用预算；AI SDK adapter 关闭内部 retry，模型模式默认关闭。
- [x] 使用独立 consumer group 消费版本化事件，通过 Event ID/Task ID 双唯一、事务 claim、lease 和精确 token 实现跨进程幂等。
- [x] 使用显式 main/retry/dead topic 实现永久错误直达死信和瞬时错误有界重试，失败发布阻止 handler 完成；真实 Kafka 验证 poison、retry→dead 与 rebalance（`AD-028`）。
- [x] migration v19 与 MySQL ModelAuditStore 持久化 Task 唯一 Run、预算快照、原子 call slot 和模型调用终态；ModelRouter 每次 provider 调用均先占 slot，跨 Kafka 重投共享 Task 上限（`AD-029` 已关闭）。
- [x] migration v20 持久化不可变 Shadow Plan 与有序结构化 Step；同一 Task 并发重放幂等收敛，plan/event 绑定漂移 fail closed。
- [x] migration v21 将 Task 与 Runtime Run 分离，并为 Step 增加 lease/token claim 和终态 CAS；同一事件的 Go Embedded 与 TS Shadow Run 可独立审计。
- [x] 首先运行 shadow consumer，只记录计划、Tool 轨迹和结果，不执行写操作。
  - [x] metadata-only shadow plan 已通过真实 Kafka 3.9 事件与重复投递验证；模型结构化 Plan/Step 已持久化。
  - [x] 通过受认证 Agent Capability RPC 执行首个 `conversation.list` 只读 Step，并持久化 claim/result/error；公开 HTTP 旁路保持禁止（`AD-030` 已关闭）。
- [x] Runtime 核心保持框架中立，Mastra、OpenAI Agents SDK 和 LangGraph.js 仅作为参考或 adapter；模型调用通过 provider-neutral `ModelRouter`，AI SDK 仅位于 adapter 边界。
- [x] Agent Runtime 独立服务完成 Vitest、TypeScript typecheck、生产构建和 Go Core 全量回归；该证据只确认当前 shadow/协议边界稳定，不改变默认关闭的生产切流门禁。
- [x] 增加 `agent-runtime/go.mod` 作为 Go/TypeScript 服务目录边界，修复 `go test ./...` 扫描 TS 依赖内嵌 Go 源码的问题；Go 全仓与 Agent Runtime 独立测试入口均通过。

### G3：Durable Task、Context 与 Memory

- [x] 使用 Temporal TypeScript SDK 实现 AgentTask 状态机、Signal、Timer、Retry、取消和恢复；input/approval deadline 到期后确定性取消并完成持久 Run。
- [x] 建立 Agent Task Timeline v1：Core owner-scoped cursor API、Runtime/Gateway 只读代理、前端默认关闭展示，以及 Task/Run/Model/Tool/Approval/Artifact 的低敏确定性事件。
- [x] 建立 Timeline repair ledger 与显式 `agent-task-timeline-repair` 运维进程；投影失败可 durable claim、重放、完成或 retry，Prometheus 观测默认关闭；真实 MySQL 故障注入已验证 retry 到 completed 和单事件收敛。
- [ ] 完成 repair worker 的 operator 灰度、告警阈值和默认生产开关；在此之前继续保留 MySQL Timeline 主存储和前端关闭状态。
  - [x] 增加 Compose profile 级隔离 smoke：校验 v49 migration、UTC 时间基准、最小权限、worker readiness、持续 replay 和 event UUID 幂等；共享环境 operator 灰度与默认生产开关仍待完成。
  - [x] smoke 增加 worker 启动前 pending intent 与启用后恢复断言，并锁定 MySQL 全局/会话 UTC；共享环境 operator 灰度、告警抓取和轮换/回滚仍待完成。
  - [x] 增加 `agent-timeline-repair-rollout` v1 只读 evidence/policy/report 与 CLI，绑定窗口、错误比例、readiness、operator、告警和回滚演练；真实共享环境采集和 operator 决策仍待完成。
- [ ] 实现 Context Compiler，按预算组合策略、任务、会话证据、检索、Memory 和 Tool Schema。
  - [x] G2 基线实现框架中立 Context Compiler v1：全局/section 预算、full/compact/omit、trust boundary、provenance manifest 和 v22 持久审计；会话检索、Memory 与 route-specific tokenizer 在 G3 继续扩展。
  - [x] 增加受认证 `conversation.read` Core RPC 与 TS Capability，统一 canonical `conversationId`，完成 direct/group target 解析、Task/Run 身份解析、Core 资源复核、消息映射和 Runtime exact scope 门禁；ModelShadowPlanner/Temporal read activity 已接入最多 20 条 `untrusted` 会话 evidence 的 full/compact 编译；完整检索编排、route-specific tokenizer 和生产上下文灰度仍待完成。
  - [x] 增加 TS Capability RPC 客户端跨语言契约测试，固定 direct/group target 解析、可信 principal 请求边界、非法 scope 拒绝和响应 target 冲突 fail-closed；分页/检索语义与生产上下文灰度继续按证据推进。
  - [x] Context Compiler v2 接入 route-aware 最大输入窗口，按最小候选 route window 扣除最大输出预算，超出请求在编译前 fail closed；旧 v1 构造保持兼容。
  - [x] 在 RPC 边界拒绝超过请求 `limit` 的消息响应，并对 `found=false` 统一执行 target 一致性校验；Planner 保留独立的 20 条/8 KiB context 预算上限。
  - [x] Context Compiler capability section 接入 Registry descriptor 的 `id/risk/requiredPermission` 低敏元数据，按允许集合和 ID 稳定排序；输入 schema 与 route-specific tokenizer 继续保留为后续门禁。
  - [x] `conversation.list/read` descriptor 增加代码拥有的受限输入 Schema 摘要，Context Compiler 将类型、范围、默认值和额外字段策略注入 trusted capability section；运行时 Zod 校验保持最终约束。
  - [x] Capability Registry 注册阶段限制 Schema 摘要关键字、`properties` 嵌套和 4 KiB 大小，并覆盖未知字段/超限回归测试，防止 descriptor 成为未治理的模型输入通道。
  - [x] Registry 注册后深度冻结 descriptor snapshot，阻止外部修改风险、权限和 Schema，并通过 descriptor 不可变回归测试。
  - [x] Context Compiler 为 selected fragment 写入 `contentSha256`，Shadow Plan 仅保存哈希和 provenance 元数据，支持低敏重放校验而不持久化 prompt 正文。
- [ ] 引入 Working、Episodic、Semantic、Procedural 和 Observational Memory，并记录来源与作用域。
  - [x] migration v29、sqlc Store 与受认证 Core RPC 建立默认关闭的 scoped Memory 读取基础；Task/Run 固定 principal、tenant、Agent 和 conversation read scope，受控 Shadow 启用后 TS 按独立预算以 `untrusted` provenance fragment 注入 Context。
  - [x] 增加 Gateway principal 派生的 owner list/revoke API、稳定分页、追加式撤销审计和默认关闭的 Pencil/Vue 管理页面；公开结果省略内部 provenance URI，自动写入保持关闭。
  - [x] 增加 Core-owned accepted candidate promotion seam：服务端重新加载候选与审核、校验 exact hash/owner/状态/30 天证据窗口，并由 sqlc/MySQL 事务写入 Memory 与 promotion receipt；没有公开 Runtime 旁路或自动写入开关。
  - [x] 增加 append-only 纠正/supersession、版本冲突、默认 shadow-only Observation/Reflection Worker、candidate ledger、append-only review ledger 和 retrieval Eval；Observation/Reflection 只产出有界候选，ledger 只保存摘要/证据/策略/哈希，review 只推进待审状态，不自动写入 Memory。证据成立后再评估 Elasticsearch hybrid/vector（`AD-035`）。
  - [x] 增加 reviewed corpus v1、双 reviewer/独立 adjudicator 门禁、低敏离线 CLI 和 owner-only source manifest loader；真实语料必须通过固定路径、权限、有效期和 corpus/review SHA-256 校验，仍不触发自动 Memory 写入。
- [ ] 实现 Event Subscription 与低成本预筛选，相关事件才创建高成本 Agent Task。
  - [x] migration v28、sqlc Store 与受认证 Core RPC 固定 Definition version/resource read scope；TS `subscription` 模式在 EventLedger、Temporal 和模型前执行 `all|message_contains_any` 确定性过滤，零匹配零 Task，多匹配稳定固定 Subscription ID，默认保持 `direct_target`。
  - [x] 增加认证 owner list/create/revoke API、版本化撤销审计、active Definition 目录、readable/scope conversation chooser 和默认关闭的 Agent 配置 UI。
  - [ ] 根据真实 reviewed corpus、Eval 与成本证据引入小模型、embedding 或向量预筛选，并完成 subscription Runtime 灰度/回切门禁（`AD-034`）。
    - [x] 增加 Memory prefilter provider-neutral evidence v1：embedding/small_model 的逐 case score/threshold、延迟和成本绑定 reviewed corpus SHA-256，并提供低敏离线聚合评测；不接入模型、Kafka 或生产灰度。
    - [x] 增加 Memory prefilter rollout decision v1：重新计算 review 与 evidence 门禁并绑定四类哈希，输出 `eligible|blocked`；仍不改变 Runtime、Kafka 或自动 Memory 写入开关。
    - [x] 增加 Memory prefilter Runtime binding v1：以 `off/shadow/enforced` 三态和候选/配置/语料/评审哈希建立可复用 gate；仅 `enforced + eligible` 允许后续任务创建，默认未接入生产。
    - [x] 增加默认关闭的 direct-target 在线 Shadow 对照、固定低基数指标、Prometheus error/drift 告警和无数据迁移回滚路径；真实 corpus 与晋级决策仍待完成。
    - [x] 增加 24 小时 Prometheus 快照 evidence Schema/CLI，固定覆盖率、样本量、counter reset、零 error、双 authority=false 与 24 小时有效期；真实共享环境归档仍待完成。
    - [x] 增加 Subscription Runtime `off/shadow/enforced` rollout gate，并接入 Kafka Shadow Runtime 可选依赖；强制模式校验 decision、candidate、corpus、review 和 evidence 的精确哈希绑定，默认保持关闭，真实 Kafka/模型灰度仍待完成。
  - [x] 增加只读 Prometheus Collector，固定 19 次历史查询、单 Agent series、全窗口 enabled 与低敏失败语义；部署 revision 仍由发布记录提供，真实共享环境未自动访问。
    - [x] Runtime matcher 在 Schema 解析前限制最多 256 条订阅候选，超限 fail-closed 并通过回归测试固定有界匹配成本。
    - [x] Subscription Shadow 记录 Core 原始候选数并覆盖 matcher error 保留计数，修正 miss 场景成本证据；共享环境窗口仍待完成。
    - [x] Subscription Shadow metrics 对 outcome 执行运行时闭集校验，防止未知低基数标签污染 evidence；共享环境窗口仍待完成。
    - [x] HTTP Prometheus Collector 对响应体实施 256 KiB 流式上限，超限或解析异常固定 fail-closed；共享环境窗口仍待完成。
- [x] 支持 `WAITING_INPUT`、`WAITING_APPROVAL` 和版本化 Artifact；产品 UI 与敏感输入隔离仍按独立门槛推进。
  - [x] `dipole.agent.elicitation.v1`、Gateway JWT API、Core Task owner 复核与 Temporal Signal 已实现持久 `WAITING_INPUT`；无效/旧 request fail closed，Worker 替换后可恢复。Pencil UI、敏感输入和 MCP adapter 由 `AD-036` 跟踪。
  - [x] migration v26 与 `dipole.agent.artifact.v1` 已建立版本化 Artifact：Temporal `read_shadow` 经受认证 Core RPC 创建 Task/Run 绑定的不可变元数据和 MinIO 正文，Gateway 读取按 Task principal 授权；更新、删除、公开 URL 与消息发送继续关闭。
- [x] Message v1 Envelope 以可选 `lineage.origin/causation_event_id/agent_task_id` 传播 Agent 因果链；Kafka consumer 滚动 causation，Embedded Agent/Outbox 保留根 Agent Task，TS Runtime 在 EventLedger、Temporal 和模型调用前抑制同源 Agent 事件，legacy v1 事件继续兼容。

### G4：MCP、评估、观测与安全门禁

- [ ] Runtime 作为 MCP Client 接入外部工具，并以 MCP Server 暴露受控 Dipole Capability。
  - [x] 使用官方 MCP TS SDK v2 建立 Client/Server foundation：只读 Capability 投影复用 Registry/Policy，宿主注入 trusted Context；Client 校验 Server identity 与双 allowlist，InMemory/Streamable HTTP 契约通过。
  - [x] 增加默认关闭的 Runtime/Gateway Streamable HTTP 挂载：Gateway JWT 固定 principal，Core 按 Task/Run 解析可信 ExecutionContext，当前只开放显式只读 Capability。
  - [x] migration v30、sqlc Store 与 additive Core RPC 建立 MCP ToolCall 持久审计；TS 执行器在 durable begin 后执行，并创建不含正文的原生 OTel span，失败与超限结果 fail closed。
  - [x] Gateway 增加独立于旧总开关的 Redis principal 限流：GET/POST 跨 Task/Run/实例共享额度，Redis 故障 fail closed，DELETE 保留 Session 清理能力。
  - [x] 增加默认关闭的 Node trace SDK + OTLP/HTTP protobuf exporter，使用标准 OTel endpoint/protocol/sampler 参数、ParentBased 比例采样、低敏 span limit 和 graceful flush。
  - [x] 增加默认关闭的 Collector + Tempo 运维 profile、24 小时 local retention、低基数告警、trace/audit runbook 和真实 trace 查询 smoke。
  - [x] 增加第一方 consent exchange 和 15 分钟 MCP 专用 JWT，以 canonical `aud`、只读 `scope`、`token_use` 及 Gateway/Runtime 双重验证阻止令牌混用；部署 URI 可统一覆盖且入口继续默认关闭。
  - [x] 增加有界 Tool timeout、cooperative AbortSignal、稳定 `tool_timeout` 审计和外部 Client request/total timeout；Gateway 断连向 Runtime 传播且 DELETE 清理保持可用。
  - [x] 增加默认关闭的外部 MCP Profile v1 与租户 Registry：配置只保存版本化 credential/CA opaque ref，严格绑定 HTTPS、Server/Tool/Host/Port/TLS identity，并把逐次公网 DNS 校验收敛到尚未注入的 Transport Factory；误开开关会 fail closed。
  - [x] 增加外部 MCP Credential Catalog v1：每次建连前按 tenant/ref/version 重新加载并校验生效窗口和 revoked 状态，只向 Factory 传递 opaque provider secret ref；原始 secret、生产 Catalog source 与 Provider 仍未启用。
  - [x] 增加受约束 Catalog file source：每次 resolve 以 `O_NOFOLLOW` 打开绝对路径，校验 regular/single-link、root/Runtime owner、group/other 不可写和有界大小，并通过原子替换传播轮换/吊销；Runtime 尚未装配该 source。
  - [x] 增加 provider-neutral MCP `AuthProvider` adapter：每次请求按 exact binding 获取 fresh secret bytes，使用独立 timeout/AbortSignal、Bearer 字符/大小校验、固定脱敏错误和 buffer wipe；不缓存 token、不提供自动 401 refresh，生产 Secret backend 仍关闭。
  - [x] 增加默认关闭的外部 MCP Network Guard：每个请求重新解析并要求全部地址为公网，把批准地址交给 pinned Dispatcher 后核对实际 peer，拒绝重定向与 rebinding；生产 Resolver/Dispatcher/Factory 仍缺席。
  - [x] 增加外部 MCP Result-to-Context adapter：成功结果以有界不可变 JSON 快照进入 `untrusted` evidence，并绑定 Profile/Server/Tool/Invocation provenance；compact 内容不复制外部正文，生产调用链仍未启用。
  - [x] 增加默认关闭的 MCP write Approval gate：Core active-only RPC 原子消费 Task/Run/Capability/Scope/Arguments/Nonce 精确绑定，TS 在 Policy/Resource 校验后消费成功才执行；生产 MCP Server 仍保持 read-only。
  - [x] migration v31 将写 ToolCall 的已消费 Approval 与完成后的 Message Command/UUID 连接为有界 action reference；Core 通过 sender-scoped receipt 复核权威 Message，审计表不保存消息正文，生产 write Tool 投影继续关闭。
  - [x] 增加 MCP Message Command Core/TS transport，并统一 Tool runner 与 Approval gate 的排序 canonical JSON；RPC 只接受已审批 running ToolCall，不能作为裸消息发送入口。
  - [x] 增加默认关闭的第一方 Message write projection：显式 active executor 串联 Approval consumption、同一 Tool Invocation、Command RPC 与 action finish；该切片未装配生产 Registry、grant resolver 和 active context。
  - [x] 增加 active-only Approval grant resolution：Core/sqlc 唯一 exact 查询返回持久摘要，TS 独立复核后连接 write gate；生产 Registry、write executor 和 active context 继续缺席。
  - [x] 增加 active ExecutionContext fail-closed seam：active Run admission 必须经过注入式 promotion authorizer，MCP context 使用 Core 持久 Run 的权威 mode；生产未注入 authorizer且公开 admission 固定 shadow。
  - [x] migration v32 增加 durable Runtime promotion grant：双人签署并绑定 candidate/Definition/promotion v2/evidence/Eval Suite；active Run 持久 candidate，每次 context resolve 重查撤销状态，生产签发与装配继续关闭。
  - [x] 增加默认关闭的 durable MCP Elicitation adapter 与单轮 MRTR continuation：受限 form 转为现有 Temporal `wait_input`，checkpoint 绑定 Request/Server/Tool/Invocation/deadline、原 Tool 参数与 opaque `requestState`；现代 Client 显式锁定 `2026-07-28` 并可在新连接中生成精确续接请求。生产 Activity/Transport Factory 装配、多轮与敏感授权仍关闭。
  - [x] 增加默认关闭的 Activity-safe round runner 与 External Profile adapter：每轮使用全新现代 Client/Transport，tenant/profile/server/tool 漂移、取消、握手失败和第二轮 input request 均 fail closed；生产 Worker mode 与外部 Provider 装配仍关闭。
  - [x] migration v35 与 Resolve RPC 持久化外部 MCP Tool command，绑定 running Task/Run/Invocation、canonical 参数摘要和无凭据 Profile/Server；Worker mode 仍需 round receipt/idempotency 处理远端成功后 Activity completion 丢失窗口。
  - [x] migration v36 与 Core/TS RPC 增加 durable MCP round receipt：确定性请求绑定、原子 Claim、owner-only terminal write、completed/failed replay 和无 reclaim 的 ambiguous fail-closed 语义已接入 Activity；生产 Worker、外部 Provider 与网络开关继续关闭。
  - [x] 增加默认关闭的 MCP Worker command dispatcher：Workflow 侧只携带 Task/Run/Invocation ID，Core 返回持久开始时间和完整权威命令；固定 request/expiry 与重启前命令复核已覆盖，生产 Worker 仍等待真实 Transport Factory。
  - [ ] 完成标准 OAuth 2.1 discovery/PKCE/客户端注册、外部 Server 凭据、生产 trace 对象存储/Alertmanager、write Tool active authority 和 Elicitation 编排接线（`AD-037`）。
- [x] 建立 outcome、trajectory、permission、retrieval 和 cost 五类离线评测。
  - [x] 增加严格语言中立 Suite/Report、稳定 SHA-256、低敏 deterministic evaluator 与 `0|1|2` CLI；promotion v2 绑定完整五类报告，v1 保持兼容。
  - [x] 使用 sqlc/TS 共享只读查询将真实 Shadow Task 转换为五类 observation；Task/Run 摘要绑定 Suite，缺失终态、指标、价格或逐 attempt 耗时证据时 fail closed。
  - [ ] 扩充人工标注 corpus、retrieval relevance、reviewer agreement 与候选成本阈值后归档生产证据（`AD-038`）。
- [x] 通过 OpenTelemetry API 记录 Task、Run、ContextCompile、ModelCall、ToolCall、Approval 和 Artifact span。
  - [x] Foundation 与 Durable Activity 使用统一低敏 `AgentTelemetry`；每个 provider attempt 和 native/MCP Tool 调用独立成 span，Temporal Workflow 保持无副作用。SDK/exporter、采样和告警由 `AD-037` 继续跟踪。
- [ ] 对 Prompt Injection、越权 Tool、敏感数据外发、重复事件和循环调用进行专项测试。
  - [x] 增加 deterministic security suite，以真实 Context、Policy/Capability、EventLedger/lineage 和 MCP Client/Server 验证 provenance、执行前拒绝、去重、循环抑制和有界 egress。
- [ ] 使用真实候选模型和人工标注 adversarial corpus 评测语义抗注入、间接注入与值级敏感信息外发（`AD-037`、`AD-038`）。
- [ ] 模型、Prompt、Tool Schema 与 Memory Policy 升级先离线评测，再 shadow，最后按用户灰度。
  - [x] 增加 `agent.release-manifest.v1`，把四类运行组件版本/哈希与 offline Eval Suite 精确绑定，并将 promotion 输入限制在 `shadow` 阶段；真实语料、共享环境观察窗口和用户灰度仍待完成。
  - [x] 将 release manifest 接入 promotion publication CLI/Artifact：携带 manifest 的新输入强制走绑定入口并持久化 manifest 哈希，旧 v2 证据回放继续兼容。
- [ ] 保留 Agent 总开关；A2A、多 Agent 与 MCP experimental Tasks 在核心门禁通过后评估。

**阶段三验收：** Agent Runtime 可独立部署和扩容；故障与升级不影响 IM Core；权限、审计、成本和效果均可观测。

详细设计见 [Agent Runtime 设计](AGENT-RUNTIME-DESIGN.md)。

## 9. 阶段四：C++ 实时数据面

该阶段只在微服务与现代存储架构稳定后启动，并与 Agent Runtime 保持独立发布。优先评估 Realtime Delivery，再依据数据决定是否替换 Gateway。

### C1：建立 Go 数据面基准

- [x] 建立 operations/baseline v4 资源采集器，按服务记录 CPU core%、采样 RSS 峰值、线程峰值和 context switch，并保留 v1-v3 兼容读取。
- [x] 在固定 20/50/100 连接梯度中归档吞吐、P50/P95/P99、CPU、RSS、context switch，并完成 node2 stop/start 故障恢复基线。
- [x] 将投递 envelope、节点批次、ACK/error、背压和热群 mode 定义为版本化 Protobuf 与跨语言 golden vectors；连接级队列、持久重试和去重在 C2 shadow 中实现。
- [x] 明确 Gateway 与 Delivery 的进程及数据所有权边界，禁止 C++ 数据面访问业务数据库。

### C2：C++ Realtime Delivery Shadow

- [x] 建立独立 C++20 contract-only foundation，在 build 目录生成 canonical Protobuf 类型，共用 golden vectors，并提供 fail-closed 配置与健康端点；暂不接入运行拓扑。
- [x] 建立无网络状态的 Kafka record 到 Delivery v1 纯投影，固定 direct/group/hot/timeline/file 与 legacy-created 语义，并以稳定 ID 支持确定性重放。
- [x] 建立独立 librdkafka shadow runtime、evidence-before-commit、assignment readiness 和低敏 NDJSON 证据；运行入口不写 Redis、Gateway 或客户端。
- [x] 完成 Kafka 消费、Redis Presence、节点级批处理、有界 observation/primary ACK、稳定 delivery ID 和背压分类；shadow 对照及 one-shot primary seam 已归档。
- [x] 增加独立 `dipole-realtime-primary-*` authority 和默认关闭的显式 primary CLI；terminal ACK/evidence 后提交，partial/error 保留 pending record，shadow 命令与证据保持兼容。
- [x] 归档真实 primary queue saturation、consume-to-ACK offset 提交、故障 retain 与进程 `SIGKILL` 重放；报告 8/8，窄 terminal evidence/commit 崩溃窗口保持未声明。
- [x] 与 Go Delivery 并行消费 shadow 流量，按同一 workload 比较投影、节点观察与最终 lag，不重复投递客户端。
- [ ] 通过压测与故障注入证明收益；C3 故障注入已通过，但 2026-08-29 projection microbenchmark 的 C++/Go ops ratio 为 `0.10`，低于 `1.0` 晋级门槛，因此保留 Go projection 并停止当前 C++ projection 替换；只有新的可复现 workload 证明收益后才重新评估。

### C3：灰度切换与 Gateway 评估

- [x] 关闭 `AD-041`：建立互斥 Go/C++ 客户端投递 authority、双 group checkpoint 和可执行自动回切，禁止两个写 authority 并行 active。
  - [x] 增加默认 `go` 的本地 `go|shadow|cpp` 配置、Gateway checkpoint-only Handler 与 C++ 启动错配门禁；保留共享 fencing 和回切证据作为后续切片。
  - [x] 在隔离 Go/C++ topology 中证明目标事件各只有一个客户端 frame，并确认 Go checkpoint group 与 C++ primary group 均达到 log end/lag 0；证据包保留应用 readiness 与临时 Compose health 探针误报诊断。
  - [x] 冻结 `epoch + authority + phase + lease expiry` 共享 fence v1，并让 Go Gateway 在启动及每条消息副作用前 fail closed 核验；默认保持关闭。
  - [x] 让 C++ shadow/primary 消费同一 fence golden vectors，在创建 Kafka consumer 及每个 pending record 投影前核验，并在拒绝时保留坐标和撤销 readiness。
  - [x] 增加 operator-driven Redis Lua CAS writer，强制 freeze 中间态、单调 epoch、精确 previous hash、幂等 transition ID 和低敏有 TTL receipt；保留身份认证与持久 receipt 加固。
  - [x] 增加 Go Gateway 稳定节点 observation：启动、空闲心跳与 readiness 写入短 TTL、lease-hash-bound 证据，写失败 fail closed，消息热路径不增加 observation 写放大。
  - [x] 增加 C++ 显式实例身份、`SET PX` observation、空 Kafka heartbeat 与独立 fence readiness；跨语言 reason vectors 和真实 Redis 刷新通过。
  - [x] 实现预期节点聚合和双 group checkpoint receipt；收据绑定短 TTL proof、transition lease hash、完整 assignment、逐分区 committed/log end 与两组一致高水位，并以不可覆盖文件持久化。
  - [x] 建立共享、租约化 authority fencing 和双 group checkpoint receipt，完成中断后确定性续切或回切。
    - [x] 增加不可变 cutover attempt manifest 与哈希链事件日志，确定性归约正常续切、冻结期直接回退和目标激活后二次冻结回退。
    - [x] 增加单步恢复 orchestrator、确定性幂等 action ID 与首次冻结超预算自动回退决策，动作失败时保持 journal 原位。
    - [x] 增加不可覆盖 action artifact envelope，独立绑定 canonical action 与外部 receipt/checkpoint payload，为模糊故障重试提供持久幂等边界。
    - [x] 接入 production transition/checkpoint executor，验证 initial lease 和全部 manifest 绑定，并覆盖 forward、两条 rollback 与 Redis receipt 恢复。
    - [x] 建立自包含 immutable attempt workspace，持久化 canonical 输入并在恢复时重算全部 manifest/lease 绑定。
    - [x] 增加 create/status/单步 advance/rollback 恢复命令，所有变更要求确认并在每个副作用后形成持久边界。
    - [x] 增加证据链内的单步 lease renew；续期不重置冻结预算，并强制重采绑定旧 lease 的节点/checkpoint 证据。
    - [x] 用隔离真实 Kafka/Redis 与 race harness 完成 controller crash、Kafka member loss/rejoin、Redis outage/recovery 的 forward cutover 演练并归档证据。
    - [x] 完成真实 expired-freeze 自动回切，强制 source-node frozen proof 后恢复 Go active epoch 2。
    - [x] 增加持续续期调度，并完成 C++ primary authority 演练。
- [ ] 按节点或用户灰度将投递切到 C++，保留 Go 回切开关和独立 consumer group。
- [x] 完成 crash isolation、重平衡、Redis 故障、慢消费者和队列溢出演练；C3 真实隔离演练覆盖 Controller/C++ 进程替换、Redis outage、Kafka member loss/rejoin、过期 freeze 自动回切和 primary 停止恢复，证据归档于 `/tmp/dipole-c3-cutover-fault-report.json` 与 `/tmp/dipole-c3-cutover-fault-report-controller.json`，报告绑定当前 revision 和依赖/二进制哈希。
- [ ] Delivery 稳定后再评估 C++ WebSocket Gateway；cgo 仅用于接口窄、批处理明确的 native codec 实验。

**阶段四验收：** C++ 实现通过同一 contract test，在目标负载下取得可复现收益，故障不会影响 Go 业务控制面，并完成自动回切演练。

## 10. 持续轨道：Pencil 前端设计

- [x] F1：建立 `design/dipole-ui.pen`、design tokens、核心组件，以及 Login/Chat desktop/mobile 设计。
- [ ] F2：Search 四态、Vue 工作区、Sync 状态矩阵、desktop/mobile 恢复稿和标题栏状态已完成；Contact、Group、File、Device 与 Settings 待完成。
- [ ] F3：Agent Workflow Repair proposal/evidence/双人 approval 和普通 Elicitation Form 七态的 desktop/mobile 设计已完成；默认关闭的 schema-driven Vue Form 已接入 authenticated Task query/input/cancel，前端 Task parser 已严格保留 `waiting_approval` pending 状态。MCP 单轮 continuation 已具备 Runtime 契约但尚未装配生产 Activity。Agent Definition、Subscription、完整 Task timeline、Memory、Artifact、多轮与敏感授权仍由 AD-036 跟踪。
- [ ] F4：已建立 Pencil 增量更新、设计日志、Vite 8/Vitest 4 工具链契约、组件测试和 Playwright IndexedDB E2E 基线；Vue token 映射、页面流程与视觉回归待完成。
  - [x] App 壳层、Login、Search 工作区、Agent Task Timeline 组件及其路由页面、Agent Event Subscription 和 Agent Memory 管理页已引用共享 `--dp-*` token，并由 Vitest 契约测试和 Timeline Playwright 流程校验 Pencil variables、路由和核心样式边界。
  - [x] Agent Approval 与 Elicitation 表单已引用共享 `--dp-*` token，并由 Vitest 设计契约测试校验主题边界；截图级视觉回归仍待完成。
  - [x] Agent Approval 页面已增加 Playwright 认证流程，校验审批绑定、fail-closed 重试和移动端单列布局；截图级视觉回归仍待完成。
  - [x] Agent Approval 与 Elicitation 已增加 Chromium canonical 截图回归，固定主要桌面布局；其余页面和真实 Pencil 增量编辑仍待完成。
  - [x] Agent Subscription 与 Memory 管理页已增加 Chromium canonical 截图回归，固定治理控制面共享 token；其余页面和真实 Pencil 增量编辑仍待完成。
  - [x] Search Workspace 已清理主题硬编码并统一共享 `--dp-*` token，补充设计契约测试；截图级 Search 视觉回归仍待完成。
  - [x] Search Workspace 已通过 E2E visual harness 固定 Chromium canonical 五态截图，覆盖 Idle、Loading、Results、Empty、Error；真实 Pencil 增量编辑和跨平台截图差异仍待完成。

Pencil CLI 增量任务本轮未形成可提交设计资产，原因和回滚边界记录在 `AD-044`；既有 canonical 文件和批准导出保持不变。
后续自动化 Pencil 编辑统一通过 `scripts/pencil-safe-edit.mjs`，先在临时路径完成并校验，再替换 canonical 文件。

当前质量基线：Agent Runtime `npm test` 通过 122 个测试文件/627 个测试；Frontend `npm test`、`vue-tsc` 与 Vite 生产构建通过。该验证不等同于 F2-F4 设计稿和真实浏览器视觉回归全部完成。
Agent Runtime 的 `npm run typecheck` 与 `npm run build` 也已通过；模型调用仍经 provider-neutral `ModelRouter` 边界。

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
| `message.timeline_notify_mode` | `off / shadow` | Gateway 轻量 Timeline 通知；`off` 立即停止附加通知且保留完整消息投递 |
| `VITE_TIMELINE_NOTIFY_MODE` | `off / shadow` | Web Timeline 通知验证；未设置或 `off` 时完全忽略该通知 |
| `sync.mode` | `legacy / compare / timeline` | 客户端同步协议迁移 |
| `search.enabled` | `false / true` | ES 故障隔离 |
| `agent.mode` | `off / embedded / shadow / remote` | Agent 抽离与灰度 |
| `VITE_AGENT_ELICITATION_ENABLED` | `false / true` | Agent 普通输入 Form 路由；默认 `false` |
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
