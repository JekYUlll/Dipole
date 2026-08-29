# 服务边界清单

本文档是 Dipole Monorepo 当前服务边界的基线。它同时描述目标边界和渐进迁移期间保留的兼容例外，避免把“独立入口”误认为“实现和数据已经完全自治”。

## 服务矩阵

| 服务 | 入口 | 对外职责 | 主要调用方 | 数据所有权 | 当前过渡例外 |
| --- | --- | --- | --- | --- | --- |
| Gateway | `cmd/services/gateway` | HTTP、WebSocket、认证上下文、限流、实时连接和消息/同步查询入口 | Client | 无业务表；Redis 实时连接状态 | 其他 Core 业务 HTTP 暂通过 Core 反代 |
| Core | `cmd/services/core` | Auth、User、Group、Contact、File、Conversation | Gateway、Message、Sync、Agent Capability | 用户、群组、联系人、文件元数据、Conversation State | 仅 embedded 模式保留消息/同步 HTTP 与 WS；remote 模式由 Gateway 直接承接 |
| Message | `cmd/services/message` | Send、History、Idempotency、Seq、Outbox、Message Store | Gateway、Core、Sync | `messages`、消息幂等、Outbox、消息 Timeline | 与 Core 暂共享 MySQL 集群，账号和表权限按迁移计划逐步收紧 |
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
- `api/proto`、`contracts`：跨服务 RPC、事件和 Agent 契约。

### 需要收敛

- `internal/service`、`internal/handler`、`internal/store` 和 `internal/app` 仍包含多个服务的组合与实现，当前属于迁移中的共享实现区。
- Search application 已迁入 `internal/services/search/application/`；该目录只依赖共享 application port、Core Capability 和 Search Index 接口。
- Core capability、Auth domain、Admin domain、Session domain、User、Contact、Group domain、File domain 与 Conversation application 已迁入 `internal/services/core/`；Auth domain 位于 `core/domain/auth`，Admin domain 位于 `core/domain/admin`，Session domain 位于 `core/domain/session`，Auth/Admin/Session application 通过明确的 User/Admin、Token、Presence 和连接踢出依赖装配，User application 通过 User/File store 和对象存储依赖装配，Contact application 通过 Contact/User store、事件、通知和系统消息依赖装配，Group domain 位于 `core/domain/group`，通过 Group/User store、事件、热群、文件、对象存储和系统消息依赖装配，File domain 位于 `core/domain/file`，通过 File metadata、Message store、Redis 分片会话和对象存储依赖装配，Core capability 使用最小查询接口，Conversation application 通过明确的 repository、事件和通知依赖装配，embedded 与独立 Core runtime 共用该边界。
- Core repository composition 已提供 `CoreProcessRepositories`，集中声明 Core 所有的用户、群组、联系人、文件、Conversation State 和 Admin store；聚合 `Repositories` 仅作为 embedded 兼容入口。
- 聚合 `Repositories` 已显式保存 Core、Message、Sync、Agent 四类 process composition，后续独立启动链应直接接收对应分组，避免重新恢复扁平跨服务依赖。
- Agent repository composition 已提供 `AgentProcessRepositories`，集中声明 Agent policy、task timeline、memory、approval、artifact、tool audit 和 readiness store；Core 仅通过兼容 RPC/port 使用必要能力。Go/Eino 兼容实现位于 `internal/services/agent/legacy/`，由 TS Agent Runtime 按发布门禁逐步接管。
- Sync application 已迁入 `internal/services/sync/application/`；该目录只依赖共享 SyncStore、Core Capability 和 Sync application port，embedded 与独立 Sync runtime 共用该装配。
- Message application 已迁入 `internal/services/message/application/`；该目录只依赖共享 MessageStore、Core Capability、事件发布 port 和 Message application port，embedded 与独立 Message runtime 共用该装配。
- Gateway HTTP handlers 已迁入 `internal/gateway/http/`，只负责认证上下文、参数校验和各 application port 的响应映射；嵌入式兼容 Server 复用同一组边缘适配器。
- 服务入口只能通过 Composition Root 装配这些实现；禁止在 Handler、Transport 或另一个服务的业务包中直接创建具体 Repository。
- 业务服务不得跨边界写入其他服务拥有的表。查询应通过 application port、RPC 或版本化事件完成。
- `cmd/tools` 的回填、对账和证据程序可以复用只读 application port，但不能成为长期服务的隐式写入口。

## 目标收敛顺序

1. 先为每个服务保留独立 application port 和 contract test。
2. 将 `internal/app` 中的 Composition Root 按 Core、Message、Sync、Search 和 Agent 责任拆分。
3. 将 `internal/service`、`internal/handler` 和 `internal/store` 中仍跨服务的文件迁入对应服务包；共享部分下沉到 `internal/platform` 或明确命名的 shared package。
4. 服务完成独立数据库账号、独立迁移 owner 和 RPC/事件调用后，再删除兼容实现。

每次搬迁必须同时更新本清单、`ARCHITECTURE-DEBT.md`、测试门禁和回滚说明。
