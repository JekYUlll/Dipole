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

### 变更

- 普通群消息按成员写入 Inbox；热群沿用 notify + pull，跳过成员级 Inbox 写扩散。
- HTTP、Kafka 与 Agent 启动路径通过统一 Composition Root 创建 Repository 与消息域 Service，消除进程内重复实例和分散的具体依赖构造。
- Runtime 在 HTTP、Kafka、Outbox 和 AI 助手初始化之间复用同一 Repository 集合，保留独立兼容构造入口供测试和渐进迁移使用。
- 服务启动默认只读校验 migration 版本，停止执行 GORM `AutoMigrate`；兼容窗口可通过 `mysql.auto_migrate=true` 临时回退。
- Composition Root 支持 `data.mysql_adapter=gorm|sqlc`；当前 `sqlc` 灰度范围包含已通过双适配契约的 AICallLog、Admin、File、User、Contact、Group 与 Conversation Repository，其余仓储保持 GORM。
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

### 迁移说明

- 可设置 `DIPOLE_DATA_MYSQL_ADAPTER=sqlc` 启用已迁移仓储，发生异常时设置为 `gorm` 并重启节点即可回切；未知配置会直接拒绝启动。
- 部署或本地启动服务前执行 `go run ./cmd/migrate -direction up`，由 `000001_baseline` 创建或接管当前 12 张业务表。
- baseline migration 会创建 `user_sync_inbox` 与 `user_sync_states`；所有消息持久化节点完成升级后，并发提交顺序保证正式生效。
- Inbox 只覆盖升级后新产生的消息；升级前历史消息继续通过现有历史/离线消息接口读取。
- 现有 `/messages/offline` 接口继续保留，客户端可以渐进迁移到 `/sync`。
- 兼容缺少 `sync_fanout` 字段的旧私聊和群聊 Kafka 事件，避免滚动部署期间漏写 Inbox。

### 验证

- 已通过 `go test ./...`、`go vet ./...` 和 `go mod verify`。
- 已通过新增同步 repository、service、handler 及消息 service 的定向 race 测试。
- 已通过 Kafka `sync_fanout` 新旧字段契约测试和幂等目标隔离测试。
- 已通过 MySQL 8.4 双事务提交顺序集成测试、`FOR UPDATE` 方言测试和 Sync 锁行回滚测试。
- 已通过 MySQL 8.4 空库升级、现有 GORM schema 接管、重复执行、baseline 回滚和 schema drift 对照测试。
- 已通过 sqlc Store 的 MySQL 8.4 提交、回滚与幂等插入集成测试。
- 已通过 AICallLog GORM/sqlc 双适配的幂等、成功更新与失败更新契约测试。
- 已通过 Admin GORM/sqlc 双适配的九项统计结果契约测试。
- 已通过 File GORM/sqlc 双适配的创建回填、查询与缺失结果契约测试。
- 已通过 User GORM/sqlc 双适配的创建、更新、筛选、批量顺序与助手 upsert 契约测试。
- 已通过 Contact GORM/sqlc 双适配的双向建交、状态保留、权限、删除与申请生命周期契约测试。
- 已通过 Group GORM/sqlc 双适配的建群回滚、成员排序、幂等追加、实际计数、更新与移除契约测试。
- 已通过 Conversation GORM/sqlc 双适配的消息幂等、未读增量/清零、初始化保护、预览、备注与排序契约测试。

### 已知问题

- HTTP handler 包全量 race 测试仍会触发现有并行测试对 `gin.SetMode` 的竞态；新增 Sync Handler 的定向 race 测试已通过。
- 会话内 `message_seq`、`read_seq`、设备级 cursor 和 Inbox 清理策略留待后续迭代。
- `users.status` 的 schema 默认值 `0` 与当前 Go 领域常量 `Normal=1`、`Disabled=2` 存在偏移，已记录为 AD-012。

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
