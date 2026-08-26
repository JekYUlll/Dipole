# 更新日志

本文档记录 Dipole 的重要功能、行为变化、兼容性说明和修复。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。项目引入正式版本后，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## 维护约定

- 日常开发统一追加到 `Unreleased`，不要直接创建临时版本章节。
- 条目描述用户或系统可观察到的变化，并标明影响范围；纯格式调整和无行为变化的重构通常无需记录。
- 分类按需使用：`新增`、`变更`、`修复`、`安全`、`弃用`、`移除`、`迁移说明`、`验证`、`已知问题`。
- 正式发布时，将 `Unreleased` 内容移动到 `## [X.Y.Z] - YYYY-MM-DD`，随后保留一个空的 `Unreleased` 章节继续滚动更新。
- 数据结构、接口兼容性或部署步骤发生变化时，必须补充 `迁移说明`；未解决但会影响开发或发布的问题写入 `已知问题`。
- 可在条目末尾附关联 Issue、PR 或提交，例如 `(#123)` 或 ``(`abc1234`)``。

## [Unreleased]

### 新增

- 增加 transport-neutral `MessageApplication`、`SyncApplication`、`CoreCapability` 和 `EventPublisher` 端口、单体 Local adapter 及数据层依赖架构测试。
- 增加用户同步 Inbox Timeline：通过 `user_sync_inbox` 按用户维护持久化 `sync_seq`，支持离线和多端增量同步。
- 增加 `GET /api/v1/sync` 接口，支持 `after_seq` 游标、分页上限、`next_seq` 和 `has_more`。
- 消息持久化、Inbox 写入与 Transactional Outbox 进入同一数据库事务，避免消息事实与同步状态分离提交。
- 增加同步链路的 repository、service 和 HTTP handler 测试，并覆盖私聊、普通群聊、热群和分页场景。
- 增加分阶段平台演进计划与架构债务台账，明确微服务、存储架构和 Agent Runtime 的分支、验收及回滚策略。
- 增加 TypeScript Agent Runtime 设计，明确 Durable Task、Capability、Context、Memory、MCP、评测和渐进迁移方案。
- 增加 GORM 到 sqlc 的渐进迁移计划，以及基于 Pencil `.pen` 的前端设计与视觉回归维护规范。
- 增加版本化 MySQL migration、独立 `cmd/migrate` runner、schema ledger 与真实 MySQL drift 测试。
- 增加固定 sqlc `v1.31.1` 的生成配置与漂移门禁、`database/sql` 事务 Store、错误映射及首组 AICallLog 类型安全查询。
- 增加 `AICallLogStore` application port、GORM 可注入 adapter、sqlc adapter 及共享 MySQL contract test。
- 增加 `AdminOverviewStore` application port 与 Admin sqlc adapter，以单条聚合查询替代九次独立统计查询，并通过 GORM/sqlc 共享契约验证。
- 增加 `FileMetadataStore` application port 与 File sqlc adapter，保持创建后的 ID/时间戳回填和缺失查询语义，并纳入统一回切开关。
- 增加 `UserStore` application port、共享 Redis/Bloom 缓存装饰器与 User sqlc adapter，覆盖创建、助手 upsert、资料更新、筛选和批量查询。
- 增加 `ContactStore` application port、共享关系缓存装饰器与 Contact sqlc adapter，覆盖双向好友关系和联系人申请生命周期。
- 增加 `GroupStore` application port、共享元数据/成员缓存装饰器与事务型 Group sqlc adapter。
- 增加 `ConversationStore` application port 与 Conversation sqlc adapter，覆盖投影 upsert、列表、初始化、备注和未读状态。
- 增加 `OutboxRelayStore` application port 与事务型 sqlc adapter，覆盖有序批量领取、过期租约回收、重试退避、发布终态和 Header 解码。
- 增加 `MessageStore`、`SyncStore` application port 与 sqlc adapters，完整覆盖 Message Store 查询、Inbox Timeline 读取和 Message/Inbox/Outbox 原子写入。
- 增加 `dipole.message.v1.MessageService` protobuf/gRPC 契约、固定版本生成门禁、结构化领域错误详情，以及实现同一 `MessageApplication` 的本地 server 与远程 client adapters。
- 增加共享 `dipole.common.v1.RequestContext`、Core Capability 与 Sync Query v1 契约，以及实现现有 application ports 的 Local server/Remote client adapters。
- 增加 Kafka schema version 兼容校验与事件契约，legacy 空版本和 v1 minor 保持兼容，未知主版本跳过业务 Handler 并直接进入 DLQ。
- 增加内部 gRPC 服务身份认证基元，客户端注入服务名与共享凭据，服务端以常量时间比较校验凭据并执行调用方 allowlist。
- 增加独立 `cmd/message-service` 运行时，拥有 Message RPC、消息持久化 Kafka consumer、Transactional Outbox Relay 和远程 Core Capability client。
- Core Capability v1 增加最小文件所有权快照，文件消息通过受认证 RPC 获取 File ID、名称、大小、类型和 URL，不暴露对象存储内部键。
- 增加仅组合 Message 与 Outbox adapters 的 `MessageProcessRepositories`，独立进程停止构造 User、Contact、Group、File、Conversation、Admin 和 AI Repository。
- 增加 `message.shadow_queries` 查询影子开关，可在 Local/Remote 任一方向异步比较历史、群增量和离线消息结果。
- 增加 `message.runtime_mode=shadow|owner`；shadow 进程拒绝全部命令且不启动 Kafka/Outbox，owner 进程承担完整消息写入职责。
- 内部 RPC 支持 TLS 1.3 双向证书认证，并将认证服务身份绑定到 protobuf `caller_service`；明文 listener/target 强制限制在 loopback。
- 增加 `message.enforce_db_permissions` 启动验收和 Message MySQL 最小授权模板，检查自有表可访问且 Core 表全部拒绝。
- 增加 Message Service 渐进部署手册，覆盖影子阶段、consumer group 交接、mTLS、验收和无数据回滚流程。
- 增加可滚动维护的性能基线，首组记录 Local 与 TLS 1.3 mTLS gRPC Message History adapter 的三轮 benchmark。
- 增加独立 `cmd/gateway`，承担公开 HTTP/WS、认证上下文、限流、连接管理、Redis Presence 与 Kafka Realtime Delivery，运行时不初始化 MySQL 或 Repository。
- 增加 IM Gateway 渐进部署手册，覆盖三进程边界、mTLS 身份、灰度验收和 `embedded` 无数据回滚。

### 变更

- 普通群消息按成员写入 Inbox；热群沿用 notify + pull，跳过成员级 Inbox 写扩散。
- HTTP、Kafka 与 Agent 启动路径通过统一 Composition Root 创建 Repository 与消息域 Service，消除进程内重复实例和分散的具体依赖构造。
- Runtime 在 HTTP、Kafka、Outbox 和 AI 助手初始化之间复用同一 sqlc Repository 集合，所有构造入口必须显式提供 `*sql.DB`。
- Kafka Topic 初始化显式覆盖主 Topic、`.retry` 与 `.dead`；Publisher 和 Transactional Outbox 同时写入 `version` 与 `schema_version` headers。
- 增加 `message.transport=local|grpc` Composition Root 开关；默认 Local，grpc 模式通过进程内 channel 执行同一 MessageApplication 契约并支持安全回切。
- Messaging Composition Root 支持注入远程兼容的 `CoreCapability`，为独立 Message Service 停止直接读取 Core Repository 建立切换边界。
- `message.transport=grpc` 升级为受认证的真实 TCP channel；Core 先开放 Capability listener 再连接 Message，避免冷启动循环等待，`local` 继续作为默认回切路径。
- Kafka handler 按 Core 与 Message 进程拆分；远程模式下 Core 停止消费 `message.*.send_requested` 并停止运行 Outbox Relay。
- 本地与远程 MessageService 统一通过 Core Capability 校验文件所有权；非所有者与缺失文件返回相同不可用语义。
- Outbox Relay 停止时等待 worker 退出，避免 Kafka publisher 关闭后仍有并发发布。
- Shadow comparison 只比较 Message v1 对外字段，忽略 `CreatedAt/UpdatedAt` 等内部字段，并按时间瞬时语义处理时区差异。
- Runtime 创建的同一套 Messaging Services 现在注入 Server 与 Kafka handlers，消除 grpc 模式下重复的 Local MessageService 和 singleflight 实例。
- 增加 `gateway.mode=embedded|remote`；默认保持单体行为，`remote` 时 Core 停止注册 WS 路由和实时投递 handler，其余 HTTP/Swagger/静态 Web 由 Gateway 代理到私网 Core。
- Kafka 实时投递 handler 与 Core 领域投影拆分，独立 Gateway 固定使用 `dipole-gateway-consumer`，避免与 Core 和 Message 的消费职责竞争。
- Core 与 Gateway 调用 Message RPC 时分别声明 `dipole-core`、`dipole-gateway`，内部证书和审计主体保持独立。
- 服务启动只读校验 migration 版本，已移除运行时 schema mutation 和 `AutoMigrate` 配置。
- Composition Root 统一使用 sqlc，已移除 `data.mysql_adapter` 兼容开关和 legacy GORM adapters。
- User Repository 的 Redis/Bloom 策略从数据库适配器中抽离，由 GORM 与 sqlc 后端共享同一缓存装饰器。
- Contact Repository 的 Redis 关系缓存从数据库适配器中抽离，由 GORM 与 sqlc 后端共享同一缓存装饰器。
- Group Repository 的 Redis/Bloom 与成员排序策略从数据库适配器中抽离，由 GORM 与 sqlc 后端共享同一缓存装饰器。
- Conversation 的消息预览规则收敛到 domain model，GORM 与 sqlc 投影复用同一文本、文件、AI 和系统消息摘要语义。
- Eino 从 `v0.8.8` 升级至 `v0.9.15`，`eino-ext/components/model/openai` 从 `v0.1.12` 升级至 `v0.1.13`。
- 更新 OpenAPI/Swagger 文档，加入同步接口及其请求、响应模型。

### 修复

- 修复并发消息事务可能造成同一用户 Sync Cursor 永久跳过迟提交消息的问题，Inbox 写入现在按用户锁行串行化。
- 修复旧群消息 Kafka 事件缺少 `sync_fanout` 时被误判为热群的问题，滚动部署期间默认保留普通群 Inbox fanout。
- 修复幂等冲突复用已有消息时可能沿用新目标收件人的问题，路由身份不一致时拒绝 Outbox/Inbox 修复。
- 修复重复创建已存在好友关系时可能用新建默认值覆盖缓存、造成缓存状态与数据库状态暂时不一致的问题；建交成功后统一失效双向缓存。
- 修复群成员追加按输入切片长度递增 `member_count` 导致重复或空成员虚增的问题，改为按数据库实际插入行数计数。
- 修复 MySQL 8.4 下同一批次重复追加新成员可能触发 `Error 1869` 的问题，追加入口先按群和用户去重。
- 固定 Conversation upsert 的 SQL 赋值顺序，先基于旧 `last_message_uuid` 计算未读，再更新最新消息字段，避免依赖 GORM map 排序。

### 移除

- 移除 legacy GORM repositories、model persistence tags、运行时 `AutoMigrate`、SQLite 方言测试以及 `gorm.io/*` 依赖。
- 移除 `data.mysql_adapter`、`mysql.auto_migrate` 和无依赖 Repository/Server/Kafka 便捷构造入口。

### 迁移说明

- Message、Inbox 与 Outbox Producer 已作为同一 sqlc 事务边界运行，避免跨连接提交。
- MySQL 运行时连接池由 `database/sql` 直接初始化，migration、sqlc repositories 与 Bloom Registry 共享同一连接池。
- Bloom Registry 改用 `database/sql` 读取用户和群 UUID，停止依赖全局 GORM 查询。
- 部署或本地启动服务前执行 `go run ./cmd/migrate -direction up`，由 `000001_baseline` 创建或接管当前 12 张业务表。
- baseline migration 会创建 `user_sync_inbox` 与 `user_sync_states`；所有消息持久化节点完成升级后，并发提交顺序保证正式生效。
- Inbox 只覆盖升级后新产生的消息；升级前历史消息继续通过现有历史/离线消息接口读取。
- 现有 `/messages/offline` 接口继续保留，客户端可以渐进迁移到 `/sync`。
- 兼容缺少 `sync_fanout` 字段的旧私聊和群聊 Kafka 事件，避免滚动部署期间漏写 Inbox。
- 独立部署先启动启用 `internal_rpc` 的 Core，再启动 `cmd/message-service`；两端通过 `DIPOLE_INTERNAL_RPC_SHARED_SECRET` 注入同一服务凭据，Gateway/Core 节点随后将 `message.transport` 切换为 `grpc`。
- Gateway 独立部署要求 `gateway.mode=remote`、`message.transport=grpc`、Kafka 和内部 RPC 同时启用；Core 改为私网 HTTP 目标，公开流量只进入 Gateway。

### 验证

- 已通过 `go test ./...`、`go vet ./...` 和 `go mod verify`。
- 已通过新增同步 repository、service、handler 及消息 service 的定向 race 测试。
- 已通过 Kafka `sync_fanout` 新旧字段契约测试和幂等目标隔离测试。
- 已通过 MySQL 8.4 双事务提交顺序集成测试、`FOR UPDATE` 方言测试和 Sync 锁行回滚测试。
- 已通过 MySQL 8.4 空库升级、重复执行、未来 migration 兼容和 baseline 回滚测试。
- 已通过 sqlc Store 的 MySQL 8.4 提交、回滚与幂等插入集成测试。
- 已通过全部 sqlc Repository 的真实 MySQL 功能契约，覆盖幂等、状态转换、排序、权限、事务回滚、租约和同步顺序。
- 已通过 sqlc 同用户并发提交顺序测试，确认 Inbox `sync_seq` 与提交顺序一致。
- 已通过 Message gRPC bufconn 契约、结构化错误往返、全量 Go、vet、race 和模块完整性测试。
- 已通过 Core 五项能力与 Sync Timeline 页面的 bufconn 往返测试，覆盖调用身份、权限结果、成员快照和持久游标映射。
- 已通过 Kafka legacy/v1 minor/v2 兼容、永久 schema 错误隔离、DLQ 诊断 header 和 Outbox schema header 测试。
- 已对 Local 与 gRPC transport 运行同一套八项 MessageApplication 行为契约，覆盖文本/文件命令、历史、热群增量和离线查询。
- 已通过内部 gRPC 服务认证集成测试，覆盖合法凭据、缺失凭据、错误密钥、未授权调用方与无效启动配置。
- 已通过 Core File Capability 的 Local/Remote 契约、非所有者隐藏测试，以及 MessageService 远程文件消息测试。
- 已通过 shadow query 契约：primary 四类命令各执行一次、shadow 命令调用为零、四类查询均参与比较，差异不改变 primary 响应。
- 已通过 TLS 1.3 mTLS 真实网络测试、非 loopback 明文拒绝、认证 caller 与 payload caller 冲突拒绝测试。
- 已通过 MySQL 8.4 临时受限账号验证：五张 Message 所需表可读取，Core `users` 表返回权限拒绝，测试账号随后删除。
- Message History mTLS gRPC loopback 三轮为 `69,339-69,618 ns/op`，低于 M4 `<1 ms/op` adapter 门槛；完整端到端 P95/P99 继续由 G0 跟踪。
- 已通过 Gateway 本地 health、真实 HTTP 反向代理、依赖门禁、Gateway/Core 独立 RPC 身份和 WS Gin 全局状态 race 测试。
- 已通过本地 Core、Message、Gateway 三进程 smoke：Gateway 注入不可达 MySQL 仍正常启动，health 返回成功，Core HTTP 代理返回预期认证响应，Core remote WS 路由关闭。

### 已知问题

- 会话内 `message_seq`、`read_seq`、设备级 cursor 和 Inbox 清理策略留待后续迭代。
- `users.status` 的 schema 默认值 `0` 与当前 Go 领域常量 `Normal=1`、`Disabled=2` 存在偏移，已记录为 AD-012。
- 独立 Message Service 已停止在代码中读取 Core Repository；当前仍与 Core 共用 MySQL schema 和数据库账号，最小数据库授权记录为 AD-015。
- M5 的非 WS HTTP、Swagger 与静态 Web 仍由私网 Core 提供，Gateway 通过反向代理暴露；后续服务拆分前必须保持 Core 不直接暴露公网。

## 发布归档

当前尚未建立正式版本标签。首次发布时，从 `Unreleased` 下沉内容并使用以下格式：

```markdown
## [X.Y.Z] - YYYY-MM-DD

### 新增

- 描述新增能力及影响范围。

### 变更

- 描述行为、依赖或接口变化。

### 修复

- 描述已修复问题及触发条件。

### 安全

- 描述安全修复；敏感细节应链接到受控公告。

### 弃用

- 描述即将移除的能力、替代方案和计划移除版本。

### 移除

- 描述已移除能力及替代路径。

### 迁移说明

- 列出数据库、配置、API、消息协议或部署顺序要求。

### 验证

- 列出本次发布实际执行的测试和检查。

### 已知问题

- 列出尚未解决且可能影响使用、开发或发布的问题。
```
