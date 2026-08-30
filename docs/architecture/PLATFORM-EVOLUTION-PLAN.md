# Dipole 平台演进计划

> 状态：计划中
>
> 基线：`7a209ae merge: record eino agentic capability assessment`
>
> 更新日期：2026-08-30

## 1. 目标

Dipole 按以下顺序完成四次独立演进，并持续维护前端设计轨道：

1. **微服务改造：** 从模块化单体渐进拆出 Gateway、Message 和 Sync 服务，Core 暂时保留 User、Group、Contact、File、Auth。
2. **架构重构：** 建立 MySQL 元数据、Kafka 事件流、Cassandra 消息存储、Elasticsearch 搜索索引和 Redis 实时状态的分层架构。
3. **Agent 化：** 将进程内 Eino AI 模块演进为 TypeScript Agent Runtime，通过事件和受控 Capability API 参与 IM 业务。
4. **C++ 实时数据面：** 在稳定协议和性能基线之上评估 Realtime Delivery 与 Gateway 替换，只在收益可复现时灰度切流。

微服务阶段内先将 GORM 渐进迁移到 sqlc；前端从当前阶段开始维护 Pencil `.pen` 设计基线，并随 IM、Agent 和数据面能力持续更新。

当前微服务 Go 全量测试已在干净 worktree 中通过：测试显式绑定版本化 `configs/config.dist.yaml`，不改变生产配置搜索路径。

整个过程采用 Strangler Fig 和事件驱动抽离，任何阶段结束时都必须存在可部署、可测试、可回滚的版本。

## 1.1 开发期部署与负载测试策略

开发期远程验证采用双环境分工，避免在资源受限的本机运行完整集群：

| 环境 | 已核验资源 | 用途 | 限制 |
| --- | --- | --- | --- |
| `remote-gpu` / LAB113 | 224 vCPU、188 GiB 内存、约 1.1 TiB 可用磁盘、4 张 RTX 4090、Docker 29.1.3 | 完整微服务 Compose、Kafka/存储实验、Agent Runtime、分级负载与故障演练 | 仅开发/隔离数据；Agent 模型和 GPU 压测必须单独标注，不能外推为生产容量 |
| `TencentCloud_01` | 2 vCPU、2 GiB 内存、50 GiB 磁盘、Docker 26.1.4 | 轻量启动、API/WS smoke、小并发回归、低资源兼容性检查 | 不承载 Cassandra/Elasticsearch 全量集群、完整可观测性或高并发基线 |
| 本机 | 16 vCPU、27 GiB 内存、根分区剩余约 19 GiB、交换区接近耗尽 | 单元测试、静态检查、镜像构建前置检查 | 暂停完整集群和大规模压测，先处理磁盘/交换区压力 |

开发期远程部署必须满足以下门禁：

- [x] 增加 `scripts/check-dev-host.sh` 开发主机 preflight：按 Remote GPU、TencentCloud 和本机 profile 检查资源、Docker 与 Compose 配置；实际远程工作目录、Compose project 和部署证据仍待执行。
- [x] 增加 `scripts/smoke-microservices-lite.sh` 与依赖闭包契约测试：TencentCloud 只验证 Gateway/Core/Message/Sync 及必要依赖，Agent、Search、Cassandra、可观测性和 C++ 保持关闭；实际远程运行证据仍待维护窗口。
- [x] 增加 `scripts/remote-dev.sh`：提交绑定同步、Remote GPU 远端构建/Smoke/Benchmark 和 project 级停止统一入口；活动登录会话默认保护，已有 GPU 任务只记录资源快照并允许 CPU/容器型开发动作并行。
- [x] 增加业务集群 Compose override：MySQL Router/InnoDB Cluster、Kafka 三节点和 Redis Sentinel 已可在独立 project 中渲染；真实业务故障切换与恢复收敛仍需运行时证据和活动会话批准。
- [x] 将 Go canonical 测试和架构静态门禁接入 `scripts/remote-dev.sh test`，允许在 Remote GPU 验证提交而不启动 Compose，降低本机测试负载；远程入口自动发现用户态 Go，显式工具链路径优先。
  - [x] 候选 `dipole-dev/<user>` 的远端 tracking ref 使用受限强制 refspec 刷新，避免 squash 合并后产生非快进警告；共享 ref 继续普通 fetch，fetch 错误 fail closed。
  - [x] 候选目录 checkout 前拒绝已跟踪修改，仅清理 SHA-256 与目标 Git blob 完全一致的未跟踪冲突；不同内容和其他冲突保留文件并 fail closed，避免测试生成物阻塞提交同步。
- [x] 将 Agent Runtime 与 Frontend 的 Node 验证接入 `scripts/remote-dev.sh node-test`；Remote GPU 在 `6f15f887` 通过 Agent `134` 个测试文件/`702` 个测试（另有 `9/30` 项预期跳过）、Frontend `41` 个测试文件/`165` 个测试、typecheck 与生产构建，且构建产物退出清理已验证。
- [x] F2 File Directory：Pencil canonical desktop/mobile/state matrix、批准导出和 `/files` 认证只读目录已建立。Core 通过 SQLC owner-scoped cursor 查询和版本化 gRPC 暴露低敏 projection；存储 URL、对象键、校验值和上传会话不跨 HTTP 边界，下载逐项重新授权。Remote GPU Node 22 在 `a29d9927` 通过 38 个前端测试文件、157 项测试、typecheck 与 production build。
- [ ] F2 Device Security：已完成 Pencil desktop/mobile/七态矩阵、认证 `/devices` 页面、严格低敏会话 projection 和按稳定 Device ID 排除自身的 `logout-others` 语义；Remote GPU Node 22 已通过前端 `40/162`、typecheck/build，并发现 Chromium/Firefox/WebKit binary 缺失。跨浏览器执行、视觉基线与真实 Presence 踢出仍待环境准备，不能将实现写为生产多设备安全控制。
- [x] 增加 `scripts/bench/http-read-load.sh` 低资源只读 HTTP 探针：固定 GET、并发/超时/预期状态码和 P50/P95/P99 输出；该探针只用于 TencentCloud 兼容性回归，不替代 Remote GPU 的完整 k6 基线。
- [ ] 使用提交绑定的不可变镜像或源码版本，记录 revision、镜像摘要、配置摘要和主机资源快照。
- [ ] 先执行 readiness、migration、服务布局、mTLS、Kafka lag 和健康检查，再开始负载测试。
- [ ] 负载矩阵至少区分轻量 TencentCloud smoke、Remote GPU 单节点基线、Remote GPU 故障演练；报告记录 CPU、内存、磁盘、网络、P50/P95/P99、Kafka lag 和错误率。
- [ ] 压测期间不使用生产凭据、不暴露管理端口；结束后仅清理本次 Compose project 的容器和卷，并保留脱敏证据。
- [ ] 任一 readiness、数据一致性、错误率或资源水位门禁失败，停止加压并回到上一配置；未取得共享环境批准前不做公网流量切换。

建议顺序：先在 Remote GPU 完成完整拓扑和基线，再将同一镜像与受限资源配置部署到 TencentCloud_01 做兼容性回归。TencentCloud_01 的结果只用于低资源行为验证，Remote GPU 的结果只用于开发阶段相对比较；两者均不替代生产容量评估。

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
- **文档可视化治理：** 基础功能与系统治理文档稳定后，使用 Mermaid 维护可版本化的流程、时序和拓扑图；跨团队评审或复杂静态图使用 draw.io 源文件与导出图，并要求图与部署、接口和回滚契约同次更新。
- **叙事随证据演进：** 每个改变服务边界、默认路径、用户流程、性能结论或 Agent 权限的切片，在合并前同步更新对应的 IM 或 Agent 学习与面试材料；简历表述、演示、证据、限制和下一步必须与实现状态一致。
- **远程资源隔离：** Remote GPU 存在 GPU 任务时，仍可启动 Dipole 的 CPU、Docker、集成测试和压力测试任务；任务必须使用隔离的 Compose project、端口、目录和资源配额，禁止停止、重置或抢占已有 GPU 进程。

## 3. 当前基线

| 领域 | 当前实现 | 演进起点 |
| --- | --- | --- |
| 应用部署 | `cmd/services/` 下的 Core、Gateway、Message、Sync、Search 独立 Go 入口，另有 TS Agent Runtime；embedded 聚合入口保留 | 单节点服务 Compose 和 MySQL Router/Kafka/Redis 业务 override 均可独立渲染，embedded 模式作为回滚路径 |
| HTTP / WS | Gateway 承担远程模式 HTTP/WebSocket，Core 仅在 embedded 模式保留对应数据面 | Gateway 已通过受认证 RPC 调用 Core、Message、Sync |
| 消息 | Message Service application + SQLC repository + Outbox | Message 负责消息事实、幂等、Seq 和事件发布，Core 通过 Capability/RPC 协作 |
| 同步 | Sync Service 管理 MySQL `user_sync_inbox`、设备 Cursor 和群 checkpoint | Cassandra hydration 可选，旧 Offline 接口继续兼容 |
| 会话 | Core Conversation Projection 消费 Kafka 事件 | Conversation State 仍归 Core，后续按压力独立扩展 |
| 消息存储 | MySQL `messages` 为当前事实源，Cassandra Timeline 支持 shadow/primary 实验 | 通过 storage-neutral Store、回退和证据门禁推进迁移 |
| 事件 | Kafka + Transactional Outbox，按服务拆分 consumer ownership | 事件版本、retry/DLQ、幂等和 readiness 门禁已建立 |
| 实时状态 | Redis Presence / PubSub / Hot Group，Go Delivery 为当前 authority | C++ Realtime Delivery 仅作为默认关闭候选 profile |
| 文件 | MinIO，文件元数据归 Core，Agent Artifact 使用独立 bucket/身份 | 保持独立对象存储，不随消息库迁移 |
| AI | Go/Eino legacy 兼容链路 + 默认受控的 TS Agent Runtime shadow/active read 能力 | 通过 promotion、Temporal、Capability 和评测门禁逐步接管 |

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

**验收：** HTTP/WS 契约不变；`scripts/check-go.sh`、race 定向测试和现有端到端测试通过；单体镜像仍可独立部署。

### M2：从 GORM 渐进迁移到 sqlc

- [x] 建立版本化 SQL migration，以空库和现有库升级测试替代运行时 `AutoMigrate`。
- [x] 引入 `database/sql + sqlc`、可复现生成命令、DBTX 事务边界和 domain mapper。
- [x] 对同一 Repository Port 建立迁移契约，按低风险到高风险逐仓储迁移。
- [x] 迁移 Message、Outbox、Sync 事务和 `FOR UPDATE` 锁，并执行真实 MySQL 并发测试。
- [x] 删除 GORM adapter、model tag、SQLite 方言测试和 `gorm.io/*` 依赖。

**验收：** 服务启动不修改 schema；SQL migration、生成漂移、Repository contract、MySQL 集成和回滚测试通过；`check-sqlc.sh` 拒绝 GORM module/import/selector 与 `AutoMigrate` 回流。

详细步骤见 [GORM 到 sqlc 迁移计划](../data/DATA-ACCESS-MIGRATION.md)。

### M3：定义远程契约但仍走本地实现

- [x] 使用 protobuf 定义 Message Command 与 History Query v1 契约。
- [x] 使用 protobuf 定义 Core Authorization 和 Sync Query 契约，并复用 `common.v1.RequestContext`。
- [x] 明确 Message RPC 错误码、超时、幂等键、分页游标和认证上下文传递规则。
- [x] 生成 Message gRPC server/client，并用 bufconn 验证 Local server 与 Remote client adapters。
- [x] 为 Local 与 gRPC adapters 建立完整共享行为契约，覆盖全部命令和查询。
- [x] Kafka Topic 增加 schema version；定义兼容、弃用和死信策略。
- [x] 增加 `message.transport=local|grpc` 配置开关；M3 阶段以 `local` 作为默认兼容路径并先用 bufconn 验证 gRPC，M4 已替换为受认证网络 channel，当前微服务 Compose 默认使用 `grpc`，`local` 仅作为 embedded/故障回切路径。

**验收：** Local 与 gRPC adapter 通过同一套 contract test；关闭 gRPC 时系统行为与 M2 一致。

### M4：抽离 Message Service

- [x] 新增 `cmd/services/message`，承接发送、幂等、消息历史、Outbox 和 Message Store 接口。
- [x] 当前单体先作为 Gateway/Core，通过受认证 gRPC 调用 Message Service，并保留 `local` 回切。
- [x] Message Service 通过 Core Capability API 校验用户、好友、群成员和收件人快照，不跨库读取这些 Core 表。
- [x] 使用异步影子请求比对 Local 与 Remote 查询响应；四类发送命令只执行 primary，影子链路禁止业务写入。
- [x] 提供按节点逐步切换 `message.transport=grpc` 的 shadow/owner 运行模式、consumer group 交接手册和 Local 快速回切能力。
- [x] 明确 Message Service 数据表所有权；远程模式下 Core 停止写 `messages` 和 `outbox_events`，独立进程只组合 Message 与 Outbox adapters。

当前部署仍共享 MySQL schema，凭据与表操作已通过 AD-015 分离：Message 使用 `message.mysql.*` 和 atomic/projector 最小账号，文件所有权通过 Core Capability，内部 RPC 通过 AD-013 完成 TLS 1.3 mTLS 与 caller 身份绑定。

**验收：** 发送、历史、文件消息、热群、幂等和 Outbox 故障场景通过；Remote 模式达到基线延迟目标；回切 Local 不需要数据回滚。

### M5：抽离 IM Gateway

- [x] 新增 `cmd/services/gateway`，只保留 HTTP/WS、认证上下文、限流、连接管理和协议适配。
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

- [x] 为 Core、Message、Gateway、Sync、Search、Search Indexer 与 migration 使用各自只包含 `/app/service` 的镜像；legacy Compose 继续保留共享镜像回滚路径。
- [x] 增加独立微服务 Compose，Core/Message/Gateway 使用 TLS 1.3 mTLS、独立 caller 与健康依赖启动；Core 使用独立本地消息启动兼容配置解除冷启动环，远程 Message ownership 仍由 AD-049 跟踪。
- [x] Gateway 不依赖 MySQL service，Core 与 Message 继续使用当前 MySQL schema，表级账号由 AD-015 跟踪。
- [x] 增加可重复 smoke，覆盖 migration、冷启动、Gateway health、Core HTTP 代理和 remote WS 所有权。
- [x] 在独立 Compose project 和新构建镜像上完成全量微服务部署 smoke；readiness、指标、TLS 1.3 mTLS、Core 代理和 remote WS ownership 通过，HTTP 探针失败具备有界重试和超时。
- [x] 基础微服务 Compose 已切换为逐服务镜像和统一 `/app/service` 入口；核心 smoke、Search profile 消息 smoke 与 Timeline repair profile 均通过，候选 Kafka ownership 和生产回滚仍按 AD-048 保持待演练。

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
- [x] 为 Inbox ownership smoke 增加不可变候选 receipt：绑定源码 revision/dirty 状态、projector/atomic 模式、非破坏性回滚动作和临时资源清理结果；默认仍不执行共享环境 Kafka ownership 切换。
- [x] 前端增加默认关闭的 IndexedDB Sync Engine，以同一事务提交消息和本地游标，恢复/重连后再显式 ACK 服务端设备 Cursor。
- [x] 增加 `shadow` 双跑模式、持久化 UUID 基线/pending 窗口和 Prometheus 聚合遥测；首批只比较两个协议语义一致的收到私聊消息。
- [x] 固化 24 小时 Web Sync 观测门禁：至少 100 个 match、零终态单边差异、零 overflow，并以 promtool 固定时序验证晋级和停止条件。
- [x] Observation Session/Evidence 工具拒绝超过当前时钟 5 分钟的未来时间，并覆盖 start/status/finalize 的时间完整性测试，避免未来 Prometheus 查询伪造观察窗口。
- [x] 增加 `package-web-sync-bundle.sh`：以干净 revision、显式模式和稳定 tar 元数据生成不可覆盖的 `web-sync-bundle.v1`，输出权限固定为 `0600`，并拒绝把归档写入源目录。
- [x] 将 Web Sync bundle 打包接入 `scripts/remote-dev.sh web-sync-bundle`；远程入口固定生成 `shadow` 候选并使用 `/tmp` 输出，不启动 Compose、不申请 GPU、不改变生产客户端开关。
- [ ] 使用候选 commit/bundle 哈希绑定的 Observation Session/Evidence 完成真实客户端观察窗口：match 样本达到门槛，grace 后 `legacy_only/sync_only/overflow` 持续为零，并归档 Prometheus 原始响应和对象版本后，再结束旧 Offline 兼容窗口。
- [x] 统一显式退出、HTTP 401、WS kick 与账号切换的 Session Termination；凭据先撤销，IndexedDB 清理等待在途同步收敛，快速重登等待旧清理完成。
- [x] 建立 IndexedDB 高低容量水位、按会话保底的最近消息安全淘汰、缓存 manifest 和 quota error 状态；淘汰与 Cursor 提交保持同一事务且不额外推进安全游标。
- [x] 建立 Playwright 三浏览器 IndexedDB 验收，覆盖淘汰、重开、账号隔离、延迟清理和页面中断事务原子性；增加 `storage_full/sync_error` 聚合指标与 promtool 告警。
- [x] 使用独立 Chromium persistent profile 在 `commitPage` pending 窗口触发完整浏览器主进程 crash；同一 profile 重启后 Message、manifest 与安全 Cursor 保持整页原子性。
- [x] 使用无特权 user/mount namespace 和 128 MiB tmpfs 触发真实 Chromium IndexedDB 容量拒绝；释放 reserve 后验证失败页不推进安全 Cursor，现有 `storage_full` 分类有效。
- [x] Web Sync observation Evidence 强制绑定受控对象存储归档收据，校验 URI、object version、ETag 和 retention 截止时间；缺失收据 fail closed，真实 24 小时客户端窗口仍待完成。
- [x] 完成真实浏览器配额、共享设备 HTTP 401/WS kick 和完整进程强退验收，关闭 `AD-025`。
- [x] Web Sync Engine 将热群补拉消息与群 `message_seq` 原子写入 IndexedDB，落库后再 ACK 设备群 checkpoint；`off` 模式保持不 ACK 的内存兼容路径。
- [x] 补齐 Direct Timeline `after_seq` 的 HTTP、Message v1 gRPC、Local/Remote/Shadow 与 Cassandra cohort/fallback 契约，使单聊和群聊共享会话 Seq 增量语义。
- [x] 增加默认关闭的 `sync.item.notify.v1` shadow 协议；通知只携带版本化 locator，现有完整 WS 正文继续投递，热群保留单一聚合 notify + pull 路径。
- [x] 打通 Gateway/WS `message.timeline_notify_mode=primary` 与 Web `VITE_TIMELINE_NOTIFY_MODE=primary` 配置契约；primary 仍只投递无正文 locator，客户端完成连续序列和 UUID 校验后补拉，服务端 Cassandra 观测门禁独立控制。
- [x] 增加 Web Timeline shadow verifier、会话级补洞/去重和有界遥测；固化完整 24 小时、至少 100 次 match、零 missing/mismatch/error/invalid 的晋级门禁。
- [x] 增加 Web Timeline notify primary 客户端路径：按通知的 `conversation_seq` 串行补拉，完成 UUID/序列连续性校验后才交付消息；`off|shadow` 保持兼容，服务端 Cassandra 主读灰度证据仍未晋级。
- [x] 修复 storage-lab Cassandra 固定端口竞争：Compose 支持动态宿主机端口，hydration/read-routing smoke 反查实际映射并已并行通过；生产主读、共享环境窗口和责任人批准仍未启用。
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

### A7：大文件上传与 MinIO Multipart 增强

**当前基线：** 文件大小超过 Web 端 `4 MiB` 阈值后，Core File 通过 `initiate -> upload part -> complete` HTTP 流程调用 MinIO 原生 S3 Multipart Upload；当前默认单文件上限为 `50 MiB`、分片大小为 `5 MiB`，上传会话和 part ETag 保存在 Redis，失败路径支持 Abort。小文件仍保留单请求 `PutObject` 路径。

- [x] 已完成 MinIO `NewMultipartUpload`、`PutObjectPart`、`CompleteMultipartUpload` 和 `AbortMultipartUpload` 的服务端链路。
- [x] 已完成文件所有权校验、分片序号/大小校验、会话 TTL、缺片拒绝、完成后再写 `uploaded_files` 和失败清理。
- [x] 完成阶段已校验每个 part 的实际字节数：前置 part 必须等于声明的分片大小，最后一个 part 必须等于文件剩余大小；Redis 新记录保存 `ETag + Size`，旧 ETag-only 会话安全拒绝完成。
- [x] 已完成前端按文件大小选择单请求或 Multipart 上传；分片失败保留服务端会话与本地文件身份，后续可安全续传。
- [x] 建立 MinIO 预签名 Multipart part URL 契约：Core 按归属会话校验 part 编号并批量签发绑定 `uploadId + partNumber` 的短期 URL；现有 Core 中转路径继续作为默认回滚路径。
- [x] Web 端接入默认关闭的预签名直传试运行：按 part 批量签发 URL，浏览器直接 PUT 到 MinIO，再经 Core 登记并核验 ETag/尺寸；失败保留会话供恢复，默认 relay 路径可立即回切。
- [x] 为预签名直传落地可运行的跨域边界：Gateway 提供默认关闭的同源 S3 PUT 代理，仅转发带完整签名的合法分片并限制请求体；开源 MinIO 的 Bucket CORS API 仍不可用，XML 策略仅作为兼容实现的部署参考。
- [ ] 将分片上传流量切换为 MinIO 预签名 URL 直传，Core 只负责初始化、签发受限 part URL、登记 ETag/尺寸、完成和取消，降低大文件对业务服务带宽与连接的占用。
  - [x] 建立 `multipart-presigned-rollout/v1` 机器可判定的晋级 receipt：同版本 24 小时窗口、直传样本、fallback/failed/expired/checksum/P95 指标、clear alert、relay 回切演练与独立 reviewer 缺一即拒绝；该门禁不改变默认 `relay`。
- [x] 增加前端有界并发、指数退避和单 part 重试；当前默认 3 路并发、最多 2 次重试，失败保留 session 供后续状态查询与续传。
  - [x] 重试仅覆盖浏览器网络异常和可恢复的预签名 `408`、`429`、`5xx`；确定不可恢复的预签名 `4xx` 立即返回，避免对对象存储发起无效重复 PUT。
- [x] 增加客户端断点恢复基础：Web 按文件指纹保存 session，恢复前通过受保护状态接口校验文件元数据并跳过服务端已确认 part；完成或失败取消后清理本地 session。
- [x] 增加可见的暂停/继续控制；暂停只停止新 part 调度，已完成 part、Redis 会话和本地文件指纹保留，继续时仍绑定用户、对象键、文件大小、内容类型和 upload ID；刷新页面后可通过既有恢复入口继续。
- [x] 增加受所有权保护的 Multipart 会话状态查询，返回已完成 part 的编号、ETag 和实际尺寸，为后续浏览器暂停/恢复跳过已完成分片提供服务端 contract。
- [x] 增加 `X-Part-SHA256` part checksum：现代 Web Crypto 可用时由客户端发送，Core 在保存 ETag/Size 前校验实际读取长度并恒时比较摘要；旧客户端缺少该头时保持兼容。
- [x] 增加整文件 SHA-256、强制 checksum 模式和完成结果校验：初始化绑定 `file_sha256`，`storage.multipart_require_checksum` 开启后 Complete 读取对象校验并在不匹配时清理；默认保持兼容模式。
- [x] 增加默认 dry-run 的 `dipole-multipart-cleanup` 运维工具：按 MinIO 发起时间筛选 `message-files/` 未完成 Multipart，输出可审计 JSON；执行模式必须显式提供 `--execute --confirm`，单个 Abort 失败不会掩盖其他结果。
- [ ] 增加 MinIO 未完成 Multipart 生命周期清理、Redis 会话过期扫描、完成/取消幂等和孤儿对象 reconciliation；指标至少覆盖 active、complete、abort、expired、retry、checksum mismatch 与耗时分位数。
  - [x] Core File Service 已接入低基数 initiate/presign/register/upload_part/complete/abort 结果与耗时指标；过期扫描、孤儿 reconciliation 和完整生命周期指标仍待完成。
  - [x] `dipole-multipart-cleanup --redis-orphans` 已增加有界 Redis meta/parts 扫描与默认 dry-run 孤儿 parts 报告，显式确认后才执行删除；MinIO upload 与 Redis 事件关联、告警和完整 reconciliation 仍待完成。
  - [x] Complete 成功后写入短期完成收据，客户端重复 Complete 可返回相同文件记录；Abort 对已取消会话幂等成功，对已完成会话拒绝，存储层重复调用保持受控。
  - [x] 增加 `dipole-multipart-cleanup --reconcile` 只读 reconciliation，按 `object_key + upload_id` 对照 MinIO 未完成 upload 与 Redis session metadata，报告两类跨存储漂移且不执行删除。
  - [x] 增加 `--reconcile-fail-on-drift` 告警门禁；显式开启时发现跨存储漂移返回退出码 `3`，默认行为和只读语义保持不变。
  - [x] 增加独立 Multipart Prometheus 规则和 promtool 测试，覆盖 operation error、checksum mismatch 与 p95 latency，低基数标签保持受控。
  - [x] `--reconcile --metrics-output` 可选输出低基数 Prometheus textfile gauges，并以同目录临时文件原子替换；默认关闭，任务新鲜度监控与 Alertmanager 联调仍待完成。
  - [x] cleanup textfile 输出增加 active、expired、aborted、failed、complete 和 duration 状态指标；`--metrics-output` 兼容 cleanup-only 运行，retry、checksum mismatch 的业务观测继续由 Core operation 指标提供。
  - [x] Core 对同一 session 重复 `partNumber` 上传记录 `upload_part` retry outcome，并保持最终结果与耗时统计独立；旧 session store 通过可选 presence 接口兼容。
  - [x] 为 retry outcome 增加按 operation 聚合的连续重试告警和 promtool firing 测试；未引入用户、文件或 session 标签。
- [ ] 将大文件上限、分片大小、并发数、URL TTL 和失败重试次数纳入版本化配置与发布清单，保留旧单请求路径作为可即时回切的兼容实现。
  - [x] 建立 `contracts/multipart-upload/v1` 策略契约、默认策略和 SHA-256 绑定的 release manifest；当前默认 `relay`，`presigned` 仅作为候选模式，契约校验保留旧路径回切要求。
  - [x] `check-multipart-policy.mjs` 以该契约比对示例配置、Go 默认配置和 Web 离线回退策略，防止大小、分片、并发、重试、TTL 或默认模式跨层漂移；受控环境覆盖仍须走独立切流审批。
- [ ] 用真实 MinIO 集成测试覆盖大文件、多 part、重复 part、乱序 part、断网重试、过期会话、Abort、Complete 幂等、权限越界和服务重启恢复；补齐网关限流与代理超时验证。
  - [x] 可选真实 MinIO 代理 smoke 已覆盖一分片 UploadPart、S3 Host 签名、ETag、Complete 和对象内容核验，并自动清理测试对象；完整故障矩阵仍待完成。
  - [x] 真实 MinIO 集成契约增加上传流中断后复用同一 part 编号重试、Complete 和对象内容校验；该测试验证中断错误不污染最终对象，完整浏览器断网、过期会话和网关限流矩阵仍待完成。
  - [x] Web Multipart 调度器支持可选 `AbortSignal`：取消会传播到 presigned PUT、relay API 和 part 重试边界；页面卸载只取消在途请求并保留可恢复 session，默认上传策略保持不变。
  - [x] Web 调度器已通过断连、限流、上游 `5xx` 与永久 `4xx` 单元矩阵验证重试分类；该证据覆盖浏览器调度逻辑，真实代理和跨网络故障仍待隔离环境验收。
  - [x] Remote GPU 真实 MinIO restart smoke 已验证首个 part 写入后服务重启、续传、Complete 和最终对象内容一致；测试使用隔离持久卷并自动清理，浏览器断网、过期会话、网关限流和跨存储矩阵仍待完成。
  - [x] Remote GPU 真实 MinIO cleanup smoke 已验证未完成 upload 的实际 listing、cutoff 选择、Abort 和清理后重新列举；测试使用隔离桶并自动清理，完整浏览器/网关/跨存储故障矩阵仍待完成。
  - [x] cleanup smoke 已覆盖 MinIO listing 收敛等待与完整对象键隔离，确认服务端实际 Abort 后无残留；生产 cleanup 的 `message-files/` 前缀和默认 dry-run 语义保持不变。
  - [x] File Service 过期 session fail-closed 回归测试覆盖 status、presign、register、upload、complete 和 abort，确认过期会话不会触发 MinIO 调用；Redis/MinIO 真实 TTL 故障矩阵仍待完成。
  - [x] Redis session store 回归测试覆盖 metadata/parts 同步 TTL、分片写入续期和 completion receipt 独立 TTL；真实 Redis/MinIO 联合故障注入仍待完成。
  - [x] 增加隔离真实 MinIO+Redis reconciliation smoke，验证匹配、missing Redis metadata 和 Redis orphan drift。
  - [x] 增加可选 Redis restart 故障注入：重启后 fail-closed 识别 metadata 缺失，并继续清理 MinIO incomplete upload；默认 smoke 不启用该注入。
  - [x] cleanup 将 MinIO `NoSuchUpload` 竞态记录为 `already_gone` 并视为已收敛；未知 Abort 错误仍 fail-closed。
  - [x] 增加指标 textfile 原子发布失败测试：目标冲突时保留原目标并清理临时文件。
  - [x] 补充 HTTP Gateway Multipart `initiate` 限流测试：限流在 Core/MinIO 调用前 fail-fast 并返回 `429`。
  - [x] 增加预签名代理上游响应超时配置与 `502` 回归测试；默认 `30s`，代理关闭时不改变 relay 路径。
  - [x] 增加预签名代理按客户端地址的文件上传限流：超限在 MinIO 代理前返回 `429`，允许请求才转发。
  - [x] 增加 fault-matrix 聚合入口，统一 Go contract、真实 MinIO/Redis reconciliation 和 Redis restart smoke。
  - [x] Remote GPU 使用官方 Prometheus `3.5.0` `promtool` 完成告警规则、firing timeline 与真实 MinIO/Redis 矩阵联合验收；Docker 镜像不可用时支持显式 `DIPOLE_PROMTOOL_BIN`，默认生产路径保持不变。
  - [x] 开发期 observability profile 接入 loopback-only Alertmanager 与 `discard` receiver，Prometheus 已配置投递目标；Remote GPU 已通过 `amtool check-config` 验证基础配置，生产通知 receiver、凭据和升级策略仍由受控部署层管理。

**验收：** 预签名直传在授权范围内完成 Multipart；暂停/恢复、重试、校验和清理可观测；MinIO 故障和客户端中断均能安全回滚到旧路径，未完成 upload 不长期占用对象存储。

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
  - [x] 服务布局门禁限制 Go/Eino legacy 仅由 embedded Kafka composition 引用，Eino module import 仅允许存在于 `internal/services/agent/legacy`；独立服务入口无法重新接入该兼容链路。
  - [x] Agent Capability RPC 的 Admission、Complete、Finish 已支持显式 `runtime_id + mode` 与 active candidate version；TS 默认 shadow，active 仍需 promotion authorizer 和后续 active Activity/写能力接线。
  - [x] 增加独占 `read_active` Temporal Activity profile：active Task 通过 Core RPC 获取权威 ExecutionContext，使用同一 runtime mode 完成 Run 终态绑定；Artifact 与写 Capability 继续保持关闭。

**验收：** Capability contract test 通过；Embedded 基线可重复评测；Agent 停机不影响传统 IM。

### G2：建立 TypeScript Agent Runtime

- [x] 新增 `dipole-agent`：TypeScript、Node.js、Fastify、Vercel AI SDK、Zod 和 Kafka。
  - [x] 建立 `services/agent-runtime/` Node 22+ foundation：Fastify 健康面、Zod trusted ExecutionContext、Go 兼容 Task ID、Capability Registry、resource-scope Policy Engine 与只读 shadow processor。
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
- [x] 增加 `scripts/check-agent-runtime-container.sh` 容器门禁，绑定 revision/created/dirty provenance，验证生产镜像裁剪、非 root `node` 用户和 foundation `/readyz`；active Runtime 仍需独立切流证据。
- [x] 增加 `services/agent-runtime/go.mod` 作为 Go/TypeScript 服务目录边界，修复 `go test ./...` 扫描 TS 依赖内嵌 Go 源码的问题；Go 全仓与 Agent Runtime 独立测试入口均通过。
- [x] 在隔离 spike 分支评估 Eino `v0.10.0-alpha.x` 的 Session、后台任务和 Memory API，输出与现有 TS Runtime/Temporal/Memory 的映射与兼容性报告；报告见 `docs/architecture/EINO-V010-ALPHA-SPIKE.md`，预发布 API 不进入默认 Go/Eino 回滚依赖。

### G3：Durable Task、Context 与 Memory

- [x] 使用 Temporal TypeScript SDK 实现 AgentTask 状态机、Signal、Timer、Retry、取消和恢复；input/approval deadline 到期后确定性取消并完成持久 Run。
- [x] 建立 Agent Task Timeline v1：Core owner-scoped cursor API、Runtime/Gateway 只读代理、前端默认关闭展示，以及 Task/Run/Model/Tool/Approval/Artifact 的低敏确定性事件。
- [x] 建立 Timeline repair ledger 与显式 `agent-task-timeline-repair` 运维进程；投影失败可 durable claim、重放、完成或 retry，Prometheus 观测默认关闭；真实 MySQL 故障注入已验证 retry 到 completed 和单事件收敛。
- [x] 增加默认关闭的交互式 Task admission 前端：认证 `/agent/tasks/new` 仅发送本地幂等键和目标文本，严格确认 accepted 绑定后跳转只读 Timeline；Pencil canonical desktop/mobile/五态创建画板、三项复用组件与 2x 导出已完成，Remote GPU Node 22 定向单元、typecheck 与 production build 已通过。共享环境控制面演练继续独立推进。
- [ ] 完成 repair worker 的 operator 灰度、告警阈值和默认生产开关；在此之前继续保留 MySQL Timeline 主存储和前端关闭状态。
  - [x] 增加 Compose profile 级隔离 smoke：校验 v50 migration、UTC 时间基准、最小权限、worker readiness、持续 replay 和 event UUID 幂等；共享环境 operator 灰度与默认生产开关仍待完成。
  - [x] smoke 增加 worker 启动前 pending intent 与启用后恢复断言，并锁定 MySQL 全局/会话 UTC；共享环境 operator 灰度、告警抓取和轮换/回滚仍待完成。
  - [x] 增加 `agent-timeline-repair-rollout` v1 只读 evidence/policy/report 与 CLI，绑定窗口、错误比例、readiness、operator、告警和回滚演练；真实共享环境采集和 operator 决策仍待完成。
- [ ] 完成 Context Compiler 的完整检索编排，按预算组合策略、任务、会话证据、检索、Memory 和 Tool Schema。
  - [x] G2/G3 已实现框架中立 Context Compiler v1/v2：全局/section 预算、full/compact/omit、trust boundary、provenance manifest、v22 持久审计、route-specific tokenizer，以及会话证据、Memory 和 Capability descriptor 的确定性编译；完整检索编排与生产上下文灰度继续独立推进。
  - [x] Context hydration 对独立授权的会话、Memory 和检索读取并行调度，记录低敏数量指标；任一读取错误在模型路由前 fail closed，未改变 retrieval 默认关闭或跨会话证据边界。
  - [x] 固定并实现 Agent 检索的 Core-mediated security boundary：Runtime 不直连 Search，Core 从权威 Task/Run 恢复 principal 与 scope；`conversation.search` 使用独立 permission、`conversation/*/read` scope 与 query/结果/正文上限，结果仅作为有界 `untrusted` evidence。默认关闭的 Runtime composition 只在 `DIPOLE_AGENT_RETRIEVAL_ENABLED=true` 时注入 Search Capability；生产 Elasticsearch、跨会话检索与完整检索编排继续关闭。
  - [x] 增加默认关闭的 `DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED`：仅在 retrieval 已启用时，从当前事件正文派生最多 256 字符查询，经 Core 受权读取最多 8 条命中并按 Context budget 作为 `untrusted` evidence 编译；检索错误在模型调用前 fail closed，关闭、无正文或预算不足保持可回退路径。
  - [x] 基础 Compose、active read 与 External MCP Shadow overlay 均显式固定 retrieval 和 retrieval-to-Context 为 `false`，`check-compose.sh` 对渲染结果断言该值，避免宿主环境变量扩张默认只读 Capability surface。
  - [x] Core 可经默认关闭的 `internal_rpc.agent_conversation_search_enabled` 装配 Search client 与持久 Agent adapter；该路径强制 mTLS，Search RPC allowlist 仅含 Gateway/Core，Core 关闭开关时不建立 Search 连接或注册 adapter。共享 Shadow evidence 与生产切流继续独立推进。
  - [x] legacy Eino 测试共享的 `AgentCapabilityV1` stub 已实现 `conversation.search` 并以编译期接口断言锁定，避免 Capability 扩展仅在全量门禁时暴露测试桩漂移。
  - [x] 增加受认证 `conversation.read` Core RPC 与 TS Capability，统一 canonical `conversationId`，完成 direct/group target 解析、Task/Run 身份解析、Core 资源复核、消息映射和 Runtime exact scope 门禁；ModelShadowPlanner/Temporal read activity 已接入最多 20 条 `untrusted` 会话 evidence 的 full/compact 编译。默认关闭的 `DIPOLE_AGENT_RETRIEVAL_ENABLED` 已将受限 `conversation.search` 注册到 AI SDK Shadow/Temporal read allowlist；检索命中编排、route-specific tokenizer 和生产上下文灰度仍待完成。
  - [x] 增加 TS Capability RPC 客户端跨语言契约测试，固定 direct/group target 解析、可信 principal 请求边界、非法 scope 拒绝和响应 target 冲突 fail-closed；分页/检索语义与生产上下文灰度继续按证据推进。
  - [x] Context Compiler v2 接入 route-aware 最大输入窗口，按最小候选 route window 扣除最大输出预算，超出请求在编译前 fail closed；旧 v1 构造保持兼容。
  - [x] 在 RPC 边界拒绝超过请求 `limit` 的消息响应，并对 `found=false` 统一执行 target 一致性校验；Planner 保留独立的 20 条/8 KiB context 预算上限。
  - [x] Context Compiler capability section 接入 Registry descriptor 的 `id/risk/requiredPermission` 低敏元数据，按允许集合和 ID 稳定排序；输入 schema 与 route-specific tokenizer 继续保留为后续门禁。
  - [x] `conversation.list/read` descriptor 增加代码拥有的受限输入 Schema 摘要，Context Compiler 将类型、范围、默认值和额外字段策略注入 trusted capability section；运行时 Zod 校验保持最终约束。
  - [x] Capability Registry 注册阶段限制 Schema 摘要关键字、`properties` 嵌套和 4 KiB 大小，并覆盖未知字段/超限回归测试，防止 descriptor 成为未治理的模型输入通道。
  - [x] Registry 注册后深度冻结 descriptor snapshot，阻止外部修改风险、权限和 Schema，并通过 descriptor 不可变回归测试。
  - [x] Context Compiler 为 selected fragment 写入 `contentSha256`，Shadow Plan 仅保存哈希和 provenance 元数据，支持低敏重放校验而不持久化 prompt 正文。
  - [x] 增加 provider-neutral `RouteTokenizerAdapter` 注入边界：真实 tokenizer 可按 route 提供稳定 ID、上下文窗口和 token 计数；未配置 route 继续使用经校准的保守 UTF-8 fallback，生产接入仍需候选模型校准证据。
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
  - [x] 增加默认关闭的 `agent-external-mcp-shadow.yml` Compose overlay，要求显式 Profile、I/O/route manifests、只读 secrets、独立 Kafka group 和 Temporal 输入；基础 Compose 保持 `foundation`，移除 overlay 即可回滚。Compose 门禁固定完整渲染和缺 Profile 拒绝，真实共享环境联调继续独立验收。
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
  - [x] 增加 Project Guardian synthetic subscription corpus：四类关注项目状态、四类干扰事件、双 reviewer agreement 和共享 evaluator 回归；规则 evidence 直接复用 production `matchEventSubscriptions`，固定为低敏 fixture，不能替代真实 production corpus。
  - [ ] 扩充人工标注 corpus、retrieval relevance、reviewer agreement 与候选成本阈值后归档生产证据（`AD-038`）。
- [x] 通过 OpenTelemetry API 记录 Task、Run、ContextCompile、ModelCall、ToolCall、Approval 和 Artifact span。
- [x] Foundation 与 Durable Activity 使用统一低敏 `AgentTelemetry`；每个 provider attempt 和 native/MCP Tool 调用独立成 span，Temporal Workflow 保持无副作用。SDK/exporter、采样和告警由 `AD-037` 继续跟踪。
- [x] 复核 Go、sqlc、Compose、架构文档和 Agent OTel 静态门禁，并通过独立 Tempo/Collector smoke 验证 trace 可查询；生产长期对象存储和告警通知仍由 AD-037 管理。
- [ ] 对 Prompt Injection、越权 Tool、敏感数据外发、重复事件和循环调用进行专项测试。
  - [x] 增加 deterministic security suite，以真实 Context、Policy/Capability、EventLedger/lineage 和 MCP Client/Server 验证 provenance、执行前拒绝、去重、循环抑制和有界 egress。
- [ ] 使用真实候选模型和人工标注 adversarial corpus 评测语义抗注入、间接注入与值级敏感信息外发（`AD-037`、`AD-038`）。
- [ ] 模型、Prompt、Tool Schema 与 Memory Policy 升级先离线评测，再 shadow，最后按用户灰度。
  - [x] 增加 `agent.release-manifest.v1`，把四类运行组件版本/哈希与 offline Eval Suite 精确绑定，并将 promotion 输入限制在 `shadow` 阶段；真实语料、共享环境观察窗口和用户灰度仍待完成。
  - [x] 将 release manifest 接入 promotion publication CLI/Artifact：携带 manifest 的新输入强制走绑定入口并持久化 manifest 哈希，旧 v2 证据回放继续兼容。
  - [x] 增加 release manifest 单步阶段转移与回滚校验，禁止 `offline` 直接跳到 `user_gray`；阶段变化仍需 operator 证据，不自动改变 Runtime 开关。
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
- [x] 提供 `scripts/check-cpp-realtime-container.sh` 容器门禁，复用 Ubuntu 24.04 Dockerfile 并绑定 revision/created/dirty provenance，覆盖宿主机 gRPC C++ 依赖缺失场景。
- [x] 在仓库自带 Ubuntu 24.04 构建镜像中复核 C++ 依赖、编译和 14 项 CTest；宿主机 gRPC C++ 缺失时保留容器构建作为可复现验证路径。
- [ ] 通过压测证明 C++ 数据面收益；2026-08-29 projection microbenchmark 的 C++/Go ops ratio 为 `0.10`，低于 `1.0` 晋级门槛，因此保留 Go projection 并停止当前 C++ projection 替换；只有新的可复现 workload 证明收益后才重新评估。

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
- [ ] 按节点或用户灰度将投递切到 C++，保留 Go 回切开关和独立 consumer group；C3 的 authority、自动回切和故障注入证据已完成，灰度发布仍待独立性能收益门禁。
- [x] 完成 crash isolation、重平衡、Redis 故障、慢消费者和队列溢出演练；C3 真实隔离演练覆盖 Controller/C++ 进程替换、Redis outage、Kafka member loss/rejoin、过期 freeze 自动回切和 primary 停止恢复，证据归档于 `/tmp/dipole-c3-cutover-fault-report.json` 与 `/tmp/dipole-c3-cutover-fault-report-controller.json`，报告绑定当前 revision 和依赖/二进制哈希。
- [ ] Delivery 稳定后再评估 C++ WebSocket Gateway；cgo 仅用于接口窄、批处理明确的 native codec 实验。

**阶段四验收：** C++ 实现通过同一 contract test，在目标负载下取得可复现收益，故障不会影响 Go 业务控制面，并完成自动回切演练。

## 10. 持续轨道：Pencil 前端设计

- [x] F1：建立 `design/dipole-ui.pen`、design tokens、核心组件，以及 Login/Chat desktop/mobile 设计。
- [ ] F2：Search 四态、Vue 工作区、Sync 状态矩阵、desktop/mobile 恢复稿、标题栏状态、Contact、Group、File、Device 与 Settings 的只读或受确认流程均已完成。Settings 已固定 canonical Pencil desktop/mobile/四态画板、批准导出、Chromium 视觉基线与 Remote GPU Firefox 功能证据，仅复用签名、同步状态、Device Security 入口和退出边界；WebKit 依赖共享宿主系统库维护窗口。Group 目录从认证会话投影派生范围、逐项读取权威群投影，热群保持 `notify + pull`，所有群管理写操作继续关闭。Device 的跨浏览器执行、像素基线与真实 Presence 踢出继续作为环境切片。
- [ ] F3：Agent Workflow Repair proposal/evidence/双人 approval、普通 Elicitation Form、Task Timeline、Definition、Subscription、Memory 和 Artifact 的 desktop/mobile/state matrix 已完成；相应默认关闭或只读 Vue 页面已按认证与低敏边界接入，Definition/Subscription/Artifact 的受控跨浏览器读取证据已归档。MCP 单轮 continuation 已具备 Runtime 契约但尚未装配生产 Activity；多轮、敏感授权、产品入口编排与其余视觉回归仍由 AD-036 跟踪。
- [ ] F4：已建立 Pencil 增量更新、设计日志、Vite 8/Vitest 4 工具链契约、Vue token 映射、核心页面流程、组件测试和 Playwright IndexedDB/跨浏览器功能回归；真实 Pencil CLI 增量编辑与 Agent Timeline Chromium 截图基线已通过小批次完成，截图级全页面视觉基线和未覆盖平台场景仍待完成。
  - [x] 增加无网络 `.pen` 结构门禁，校验 canonical 设计变量、核心 desktop/mobile frame、可复用组件和 placeholder/未命名节点；该门禁不替代 Pencil 视觉评审。
  - [x] App 壳层、Login、Search 工作区、Agent Task Timeline 组件及其路由页面、Agent Event Subscription 和 Agent Memory 管理页已引用共享 `--dp-*` token，并由 Vitest 契约测试和 Timeline Playwright 流程校验 Pencil variables、路由和核心样式边界。
  - [x] Agent Approval 与 Elicitation 表单已引用共享 `--dp-*` token，并由 Vitest 设计契约测试校验主题边界；截图级视觉回归仍待完成。
  - [x] Agent Approval 页面已增加 Playwright 认证流程，校验审批绑定、fail-closed 重试和移动端单列布局；截图级视觉回归仍待完成。
  - [x] Agent Approval 与 Elicitation 已增加 Chromium canonical 截图回归，固定主要桌面布局；其余页面和真实 Pencil 增量编辑仍待完成。
  - [x] Agent Subscription 与 Memory 管理页已增加 Chromium canonical 截图回归，固定治理控制面共享 token；其余页面和真实 Pencil 增量编辑仍待完成。
- [x] Agent Task Timeline、Agent Definition Catalog 与默认关闭的 Agent Task Create 已增加 Chromium canonical 截图回归，分别固定低敏任务 metadata/provenance、精确 Definition/version/scope 的只读边界，以及初始目标表单的无 Runtime/Tool/外部服务提示；其余页面与浏览器截图基线仍待完成。
  - [x] File Directory 已增加 Chromium canonical 截图回归，固定 owner-scoped 文件 metadata、逐项重新授权下载入口和对象存储信息披露边界；对象存储、上传写路径和其余浏览器视觉回归继续待独立验证。
  - [x] Search Workspace 已清理主题硬编码并统一共享 `--dp-*` token，补充设计契约测试；截图级 Search 视觉回归仍待完成。
  - [x] Search Workspace 已通过 E2E visual harness 固定 Chromium canonical 五态截图，覆盖 Idle、Loading、Results、Empty、Error；真实 Pencil 增量编辑和跨平台截图差异仍待完成。

Pencil CLI 已通过 Agent Timeline 小批次形成可提交设计资产，原子替换、结构门禁、批准导出和 Chromium 页面截图均已完成；其余页面、完整截图级视觉基线和跨平台差异继续由 `AD-044` 跟踪。
后续自动化 Pencil 编辑统一通过 `scripts/pencil-safe-edit.mjs`，先在临时路径完成并校验，再替换 canonical 文件。

当前质量基线：Agent Runtime `npm test` 通过 125 个测试文件/665 个测试，另有 7 个文件/27 个测试按条件跳过；Frontend Vitest 通过 28 个文件/104 个测试，`npm run typecheck`、Vite 生产构建和 Chromium/Firefox/WebKit Playwright 功能回归通过。该验证与 Agent Timeline Pencil 增量资产不等同于 F2-F4 全部页面、全页面截图视觉基线和未覆盖平台场景完成。
Agent Runtime 的 `npm run typecheck` 与 `npm run build` 也已通过；模型调用仍经 provider-neutral `ModelRouter` 边界。

设计轨道不阻塞后端内部重构；任何用户可见功能进入实现前，必须先完成对应 `.pen` frame 和状态评审。详细步骤见 [Pencil 前端设计计划](../frontend/FRONTEND-DESIGN-PLAN.md)。

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

每个里程碑都需要更新 `../../CHANGELOG.md`、本计划状态和 `ARCHITECTURE-DEBT.md`，并保存测试与迁移证据。

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
| `storage.presigned_upload_proxy_enabled` | `false / true` | OSS MinIO 预签名 Multipart 的 Gateway 同源代理；默认关闭，异常时回到 Core 中转 |
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

开发切片执行、验证分层、planning-with-files 模式和 worktree 生命周期遵循 [开发工作流与提速规则](../operations/DEVELOPMENT-WORKFLOW.md)。活动计划只承载当前阶段和下一切片，历史证据进入 `progress.md`、更新日志和架构债务台账。

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
