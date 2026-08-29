# 服务边界清单

本文档是 Dipole Monorepo 当前服务边界的基线。它同时描述目标边界和渐进迁移期间保留的兼容例外，避免把“独立入口”误认为“实现和数据已经完全自治”。

## 服务矩阵

| 服务 | 入口 | 对外职责 | 主要调用方 | 数据所有权 | 当前过渡例外 |
| --- | --- | --- | --- | --- | --- |
| Gateway | `cmd/services/gateway` | HTTP、WebSocket、认证上下文、限流、实时连接和消息/同步查询入口 | Client | 无业务表；Redis 实时连接状态 | 其他 Core 业务 HTTP 暂通过 Core 反代 |
| Core | `cmd/services/core` | Auth、User、Group、Contact、File、Conversation | Gateway、Message、Sync、Agent Capability | 用户、群组、联系人、文件元数据、Conversation State | 仅 embedded 模式保留消息/同步 HTTP 与 WS；remote 模式由 Gateway 直接承接，系统消息通过受限 Message RPC |
| Message | `cmd/services/message` | Send、History、Idempotency、Seq、Outbox、Message Store | Gateway、Core、Sync | `messages`、消息幂等、Outbox、消息 Timeline | Core Capability 采用惰性 RPC 连接并由就绪探针验证；与 Core 暂共享 MySQL 集群，账号和表权限按迁移计划逐步收紧 |
| Sync | `cmd/services/sync` | User Inbox、Device Cursor、Group Checkpoint | Gateway、Message、Core | `user_sync_inbox`、设备/群同步状态 | Cassandra hydration 仍为可选 primary，MySQL 保留回退 |
| Search | `cmd/services/search` | 受权限约束的消息搜索 | Gateway | 无事实消息表；只读 Elasticsearch | Core-derived scope 通过 Capability RPC 获取 |
| Search Indexer | `cmd/services/search-indexer` | 消费事件并写入搜索索引 | Kafka | Elasticsearch 索引和 tombstone | 与 Search 共享 Elasticsearch 集群但使用不同 Alias/写入职责 |
| Agent Runtime | `services/agent-runtime` | Agent Task、Memory、Tool、Approval、Artifact | Kafka、Gateway、Core Capability | Agent 任务和记忆元数据、Artifact 元数据 | Go/Eino 兼容链路仍保留，TS Runtime 按发布门禁逐步接管 |
| Realtime Delivery | `services/realtime-delivery` | 高吞吐实时投递候选数据面 | Kafka、Redis、Gateway | 无消息事实表 | C++ profile 默认关闭，Go 仍是默认 authority |

## 共享层规则

### 允许共享

- `internal/application`：跨进程契约对应的 Go application port 和 adapter 接口。
- `internal/model`、`internal/dto`：经过版本兼容约束的领域数据结构。
- `internal/config`、`internal/logger`、`internal/middleware`：基础运行时能力。
- `internal/platform`：数据库连接、Kafka、Redis、对象存储和 RPC/TLS 等基础设施适配。
- `internal/platform/cassandra`：跨 Message/Sync 复用的 Cassandra Timeline 与 hydration 存储适配器，不承载服务业务编排。
- `internal/platform/storage`：对象存储、Search Archive 以及 MySQL/Cassandra 灰度 routing、shadow 和 hydration fallback 适配器；通过配置关闭即可回到主存储路径。
- `internal/platform/elasticsearch`：Search 与 Search Indexer 共用的版本化索引、Alias 和 mutation adapter；不保存消息事实和授权事实。
- `internal/platform/mysql`：基于 database/sql + SQLC 的共享 MySQL 连接初始化、事务边界、generated 输出和 mapper；业务仓储由各服务拥有，旧 `internal/data/mysql` 兼容目录已退役。
- `internal/platform/cache`：Redis 单节点/Sentinel 客户端、共享缓存和实时状态原语；业务服务直接依赖该平台包，旧 `internal/store` Redis 入口已退役。
- `internal/platform/runtime`：跨服务 metrics 生命周期、依赖 readiness 探针和 RPC serving 绑定；不承载业务编排、数据访问或具体服务 RPC 语义。
- Search Indexer bootstrap 直接拥有其长期运行时装配；Kafka consumer、Elasticsearch index 和服务 metrics/readiness 的启动顺序由 `internal/services/search-indexer/bootstrap/` 负责，平台包仅提供基础设施能力。
- `api/proto`、`api/gen/go`、`contracts`：跨服务 RPC 源契约、生成类型、事件和 Agent 契约；生成代码由协议目录统一维护。

### 需要收敛

- 旧 `internal/store` MySQL/Redis 入口已在全仓调用审计后退役；旧 `internal/service` 和共享 `internal/handler` 实现已清空，兼容回归测试统一收纳到 `internal/compat/service/`；embedded 聚合装配已迁入 `internal/bootstrap/embedded/`，`internal/app` 仅保留兼容测试与仍有调用者的聚合入口，生产服务入口不得直接依赖兼容目录。
- embedded 聚合专属的 Kafka 注册、Conversation projection、群初始化、旧 Eino 触发和实时投递组合位于 `internal/bootstrap/embedded/`；独立 Gateway/Core/Message 服务仍由各自 service-owned Kafka infrastructure 持有。
- `internal/operations/` 收纳回填、对账、归档和受控切换等一次性操作；Search 运维装配已从 `internal/bootstrap/` 移至 `internal/operations/search/`，长期服务启动包不得重新承载这些操作。
- Sync baseline/replay/reconcile 与 Cassandra backfill/archive/reconcile 已分别收纳到 `internal/operations/sync/` 和 `internal/operations/cassandra/`；Sync 长期 runtime 位于自身 bootstrap，Cassandra Projector runtime 归属 Message bootstrap。
- Agent Memory lineage backfill 已收纳到 `internal/operations/agent/`；manifest、审批和执行回执仍由 Agent 运维工具管理，Agent Runtime 长期实现保持在 Agent service 边界内。
- Search application 已迁入 `internal/services/search/application/`；该目录只依赖共享 application port、Core Capability 和 Search Index 接口。
- Search Index SQLC repository 及契约测试已迁入 `internal/services/search/infrastructure/mysql/`，由 Search Indexer 服务独占；共享 SQLC 基础设施仅位于 `internal/platform/mysql/`。
- Search Indexer Kafka Projector 已迁入 `internal/services/search/infrastructure/kafka/`，直接复用 Message domain 的事件 contract；旧 `internal/projector/search/` 路径由结构门禁阻止回流，索引失败仍遵循 Kafka retry/DLQ 和 Alias 回滚策略。
- Core capability、Auth domain、Admin domain、Session domain、User domain、Contact domain、Conversation domain、Group domain 与 File domain 已迁入 `internal/services/core/`；Auth domain 位于 `core/domain/auth`，Admin domain 位于 `core/domain/admin`，Session domain 位于 `core/domain/session`，User domain 位于 `core/domain/user`，Contact domain 位于 `core/domain/contact`，Conversation domain 位于 `core/domain/conversation`，Auth/Admin/Session application 通过明确的 User/Admin、Token、Presence 和连接踢出依赖装配，User application 通过 User/File store 和对象存储依赖装配，Contact application 通过 Contact/User store、事件、通知和系统消息依赖装配，Group domain 位于 `core/domain/group`，通过 Group/User store、事件、热群、文件、对象存储和系统消息依赖装配，File domain 位于 `core/domain/file`，通过 File metadata、Message store、Redis 分片会话和对象存储依赖装配，Core capability 使用最小查询接口，Conversation domain 通过明确的 repository、事件、通知和投影观察依赖装配，embedded 与独立 Core runtime 共用该边界。
- Core repository composition 已提供 `ProcessRepositories`，集中声明 Core 所有的用户、群组、联系人、文件、Conversation State 和 Admin store，并与 SQLC repository、缓存适配器共同位于 `internal/services/core/infrastructure/mysql/`；embedded 聚合通过显式 Core/Message/Sync/Agent process 分组适配，扁平 Agent 字段已移除。
- Core 专属 sqlc MySQL repository 及契约测试已迁入 `internal/services/core/infrastructure/mysql/`，Core 数据访问实现由 Core process 独占。
- Core 已提供独立 Composition Root `InitializeCoreService`：remote 模式只装配 Core-owned repository、Core messaging/application ports、Core projection、Core HTTP 和 Core Capability RPC；embedded 模式通过显式边界适配继续使用聚合入口作为本地兼容和回滚路径。
- Core 的 HTTP/WS server、静态资源和通知适配器位于 `internal/services/core/server/`，由 Core bootstrap 与 embedded runtime 共用；横向 `internal/server/` 不再作为实现目录。
- 聚合 `Repositories` 已显式保存 Core、Message、Sync、Agent 四类 process composition；Core、Agent、Message、Sync 仓储访问已统一通过对应 process 分组，Search 由独立 Elasticsearch runtime 装配，聚合层仅保留 process composition 指针，避免重新恢复扁平跨服务依赖。
- Agent repository composition 已提供 `ProcessRepositories`，集中声明 Agent policy、task timeline、memory、approval、artifact、tool audit 和 readiness store，并与 SQLC 实现共同位于 `internal/services/agent/infrastructure/mysql/`；Core 仅通过兼容 RPC/port 使用必要能力。Go/Eino 兼容实现位于 `internal/services/agent/legacy/`，由 TS Agent Runtime 按发布门禁逐步接管。
- Agent 专属 sqlc MySQL repository 及 contract tests 已迁入 `internal/services/agent/infrastructure/mysql/`，Agent 数据访问实现由 Agent process 独占。
- Agent application 的审批、审批授权、任务控制、Definition Catalog、Memory Candidate Promotion、Task Workflow Projection、MCP readiness、MCP tool round、tool audit、Runtime promotion evidence、Workflow repair audit、Artifact、Memory Owner、Subscription、Capability、Command、Workflow repair prepare、Workflow repair executor、Execution Policy、MCP tool terminal、Memory、Message command execution、Runtime promotion control 和 Runtime promotion 实现已迁入 `internal/services/agent/application/`；`internal/app` 的 Agent application 兼容 facade 已在调用者迁移后移除，服务入口和契约测试直接依赖 Agent application 包。
- Sync application 已迁入 `internal/services/sync/application/`；该目录只依赖共享 SyncStore、Core Capability 和 Sync application port，embedded 与独立 Sync runtime 共用该装配。
- Sync domain 实现已迁入 `internal/services/sync/domain/`；Sync Timeline、设备 Cursor 和群组 checkpoint contract 由服务自有 domain 持有，兼容目录仅保留跨版本 domain-event decoder 辅助。
- Sync MySQL repository、hydrator、projection 和 process composition 已迁入 `internal/services/sync/infrastructure/mysql/`；Sync 独立 runtime 与 embedded 兼容入口均通过服务专属 composition，旧共享 repository 仅保留兼容入口。
- Sync Kafka Projector 已迁入 `internal/services/sync/infrastructure/kafka/`，直接复用 Message domain 的事件 contract；旧 `internal/projector/sync/` 路径由结构门禁阻止回流，Inbox 写责任仍遵循 atomic/projector 可回滚开关。
- Sync 独立 runtime 已直接装配 Sync infrastructure composition，`internal/app` 仅保留 embedded 聚合兼容入口；Inbox 查询、checkpoint 和 hydration contract 保持兼容。
- embedded 聚合的 composition 位于 `internal/bootstrap/embedded/`，runtime 与生命周期位于其 `runtime/` 子包；共享 `internal/bootstrap` 根目录不再持有生产实现，仅保留迁移期 contract fixture。
- Inbox ownership 配置要求：Message `projector` 模式必须与启用的 Sync projector 和 Kafka 一起发布；`atomic` 模式保留为立即回滚路径，配置校验在连接副作用前 fail closed。
- Message application 已迁入 `internal/services/message/application/`；该目录只依赖共享 MessageStore、Core Capability、事件发布 port 和 Message application port，embedded 与独立 Message runtime 共用该装配。
- Message event contract 与 Sync projection 已迁入 `internal/services/message/domain/`；`send_requested` 持久化 Kafka handler 已迁入 `internal/services/message/infrastructure/kafka/`，事件版本、Mutation、Search 和 Inbox locator contract 由 Message domain 持有。
- Message MySQL repository 已迁入 `internal/services/message/infrastructure/mysql/`；`internal/services/message/infrastructure/kafka` 负责 Outbox relay，`internal/platform/mysql/generated` 与事务 Store 仍作为基础设施共享，`messages`、Metadata、Outbox 和可选 Inbox 原子写入由 Message process 组合。
- Message Cassandra Projector 的 projection 与 runtime 已分别归属 `internal/services/message/infrastructure/cassandra/` 和 `internal/services/message/bootstrap/`；`cmd/tools/cassandra-projector` 继续作为可选独立入口，Cassandra shadow/primary 开关和 MySQL 回退语义保持兼容。Message RPC server/client 由 Message、Gateway 和 embedded 自有 bootstrap 持有，embedded-only Message transport/shadow 位于 `internal/bootstrap/embedded/`。
- Message 独立 runtime 已直接使用 Message infrastructure composition、Message application factory 和自有惰性 Core Capability adapter；`internal/app` 仅保留 embedded 聚合兼容入口，独立 Message 启动不再依赖聚合 repository composition。
- embedded Message runtime 直接调用 Message-owned SQLC repository constructor；`NewRepositories` 仅负责 embedded 回滚组合，Message repository wrapper 已退休。
- Gateway HTTP handlers 已迁入 `internal/gateway/http/`，只负责认证上下文、参数校验和各 application port 的响应映射；嵌入式兼容 Server 复用同一组边缘适配器。
- Gateway Kafka consumer 使用 `internal/application` 中的版本化群组、会话、联系人和已读事件 contract；Gateway 不直接依赖 Core domain decoder，Core 负责事件生产与自身 projection，结构门禁阻止跨服务 domain 实现回流。
- Gateway Agent MCP proxy 使用 `internal/application` 中的 resource/scope contract 和安全 URL 校验；Core 负责 token issuer/verifier 与可配置 resource，Gateway 不复制认证领域实现。
- Agent MCP resource 默认值和配置解析由 `internal/application` 提供；Gateway bootstrap、middleware 和 proxy 共享同一 contract，Core Auth 仅负责 token issuer/verifier。
- 共享 authentication middleware 通过 `internal/application` 的最小 token resolver/session contract 工作；Core Auth 的 JWT 实现可替换，Gateway 认证边界不持有 Core domain 具体类型。
- WebSocket Authenticator 通过 `internal/application.TokenSessionResolver` 获取会话，transport 层只依赖认证 contract；Core Auth 继续负责 JWT verifier 和 token state 校验。
- Gateway bootstrap 通过 `gateway.NewServerWithDependencies` 显式注入 `application.TokenResolver`；Gateway Server 只依赖 contract，Core verifier 的实例化位于 Composition Root。
- Gateway HTTP/WS server、Agent 控制代理和 Search 边缘适配已归属 `internal/services/gateway/server/`；`internal/gateway/http/` 仅保留可复用的 Gin response/handler adapter，Core embedded server 继续通过显式依赖复用。
- 服务入口只能通过 Composition Root 装配这些实现；禁止在 Handler、Transport 或另一个服务的业务包中直接创建具体 Repository。
- 服务入口优先依赖自身的 `internal/services/<service>/bootstrap`；尚未完成运行时基础设施拆分的服务，可以通过该目录的兼容 facade 过渡，但入口不得直接引用共享 `internal/bootstrap`。
- `internal/data/mysql` 及其历史 repository facade 已完成调用审计并退役；具体 SQLC repository 必须位于其服务的 infrastructure 边界，跨服务共享能力仅通过 `internal/platform/mysql/` 提供。
- 业务服务不得跨边界写入其他服务拥有的表。查询应通过 application port、RPC 或版本化事件完成。
- `cmd/tools` 的回填、对账和证据程序可以复用只读 application port，但不能成为长期服务的隐式写入口。

## 目标收敛顺序

1. 先为每个服务保留独立 application port 和 contract test。
2. 将 `internal/app` 中的 Composition Root 按 Core、Message、Sync、Search 和 Agent 责任拆分。
3. 将 `internal/service` 中仍跨服务的文件迁入对应服务包；共享部分下沉到 `internal/platform` 或明确命名的 shared package。
4. 服务完成独立数据库账号、独立迁移 owner 和 RPC/事件调用后，再删除兼容实现。

每次搬迁必须同时更新本清单、`ARCHITECTURE-DEBT.md`、测试门禁和回滚说明。
