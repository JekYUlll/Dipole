# GORM 到 sqlc 迁移计划

本文档定义 Dipole 从 GORM 渐进迁移到 `database/sql + sqlc` 的边界、顺序、门禁和回滚方案。

## 1. 目标

- 使用版本化 SQL migration 维护数据库结构，停止依赖运行时 `AutoMigrate`。
- 使用 sqlc 生成 Go 侧类型安全查询，手写 SQL 成为可审查的数据访问契约。
- 保持 Service 和 Application Port 稳定，逐个 Repository 替换实现。
- 为 Go、TypeScript 和 C++ 服务共享 schema、事件与 RPC 契约。

多语言统一的锚点是版本化 SQL schema、查询语义、事件和服务 API。sqlc 负责生成 Go 数据访问代码；TypeScript 与 C++ 通过各自 driver 或 RPC 访问其拥有的数据，禁止多个服务跨语言直接共享同一业务表。

## 2. 当前基线

- 所有生产 Repository 已使用 `database/sql + sqlc`，schema 由版本化 migration 管理。
- Domain model 只保留 API JSON tags，数据库 nullable 与字段映射集中在 mapper。
- Message、Sync Inbox 与 Outbox Producer 共用 sqlc 事务，按用户排序获取 `FOR UPDATE` 锁。
- Redis/Bloom cache policy 位于 Composition Root 装饰器，数据库 adapters 只负责持久化。

## 3. 目标目录

```text
db/
├── migrations/          versioned up/down SQL
├── queries/
│   ├── user.sql
│   ├── message.sql
│   ├── sync.sql
│   └── ...
└── schema/              sqlc input schema snapshot

internal/data/mysql/
├── generated/           sqlc generated code
├── tx.go                transaction boundary
├── mapper/              generated row to domain model
└── repository/          application port adapters
```

生成代码禁止承载业务规则。Domain model 不添加数据库 tag；nullable、枚举和时间转换集中在 mapper。

## 4. 渐进步骤

### D1：冻结 schema 与替换 AutoMigrate

- [x] 从当前 MySQL 结构生成基线 migration，并在空库和现有库上验证。
- [x] 引入独立 migration runner，部署先执行 migration，再启动应用。
- [x] 校验 migration 顺序、重复执行、回滚和与当前 GORM schema 的 drift。
- [x] 完成兼容窗口后删除 `AutoMigrate` 开关与运行时 schema mutation。

当前操作顺序：

```bash
go run ./cmd/tools/migrate -direction up
go run ./cmd/services/core
```

应用默认只读校验 `schema_migrations` 版本，不在启动阶段修改 schema。baseline down 会删除全部业务表，仅允许在一次性测试库中显式使用 `-allow-destructive`。

### D2：建立 sqlc 基础设施

- [x] 固定 sqlc `v1.31.1` 配置、生成命令和生成代码检查策略。
- [x] 建立 `DBTX`、事务 helper、错误映射和真实 MySQL 测试 fixture。
- [x] 迁移期以 GORM/sqlc 共享契约验证行为，退役后保留 sqlc 功能契约。
- [x] 提供生成漂移门禁，执行 `sqlc generate` 后检查工作区无差异。
- [x] 将 SQLC-only 回流防护纳入同一门禁：拒绝 `gorm.io/*` 与历史 GORM module、任意 Go 代码中的 GORM selector/import，以及运行时 `AutoMigrate` 调用；该扫描覆盖测试代码，避免测试 fixture 保留第二套数据访问模型。

首批 AICallLog、Admin、File、User、Contact、Group、Conversation、Message、Sync 与 Outbox GORM/sqlc adapters 已通过各自的真实 MySQL contract test。Admin sqlc adapter 使用单条聚合查询替代九次独立 count；File、User、Contact 与 Group sqlc adapters 在写入后回读记录，保持 ID 与时间戳回填语义。User、Contact 与 Group 的 Redis/Bloom 策略已抽离为共享装饰器，数据库 adapters 只负责持久化。Group sqlc adapter 通过 `mysql.Store.WithinTx` 保持群、成员和成员计数原子更新；Conversation adapter 使用显式赋值顺序保证消息幂等与未读计算；Outbox Relay adapter 在同一事务内执行 `FOR UPDATE SKIP LOCKED` 领取和租约写入。GORM 会回填 AICallLog 输入模型的自增 ID，sqlc adapter 保持输入不变；调用方按单次调用创建独立日志对象，不依赖该副作用。

Composition Root 现在只接受 `*sql.DB` 并创建 sqlc adapters；迁移期的 `data.mysql_adapter` 开关已删除。

Message、Sync Inbox 与 Outbox Producer 已按一个事务边界整体接入 sqlc。收件人 UUID 在加锁前统一去重排序，逐用户创建并锁定 `user_sync_states`，随后写入 Inbox；Outbox 数据错误会回滚 Message、Sync State 和 Inbox。Relay 消费侧复用同一 sqlc transaction Store。

TypeScript Agent Runtime 的 EventLedger、Model Audit 和 Shadow Trajectory 查询分别以 `db/queries/agent_event_ledger.sql`、`db/queries/agent_model_audit.sql` 与 `db/queries/agent_shadow_trajectory.sql` 为唯一来源：sqlc 校验 MySQL schema 与命名查询并生成 Go 契约，对应生成器从相同文件产出 mysql2 使用的 TypeScript 常量。Task/Definition/Run 继续由 Go sqlc `AgentPolicyStoreV1` 独占写入，TS 经受认证 Agent gRPC admission/lifecycle 访问，Task query/cancel/approval 控制也必须先调用 Core `AuthorizeTaskControl` 校验 principal 所有权。TS adapter 不维护第二份手写 SQL；其账号仅访问 Agent 自有 ledger、模型审计和 Shadow 轨迹表，不能直接读取或写入 `agent_tasks`。

`000022_agent_context_manifest` 在现有 Shadow Plan 上追加 nullable compiler version、估算 Token 和 provenance manifest。旧 Runtime 可继续写 NULL；新 Runtime 将 manifest 与 Plan 一起原子插入，sqlc 与 TS mysql2 常量继续由同一命名查询生成。

`000024_agent_task_workflow_projection` 在 Core 所有的 `agent_tasks` 上追加整体 nullable 的 Workflow ID、Run ID、状态、单调 revision 与更新时间。TS Runtime 只能通过受认证 `ProjectTaskWorkflowState` RPC 提交状态；Go 服务验证 Task/Run/runtime/mode 与确定性 Workflow ID 后调用 sqlc CAS。原 `agent_tasks.status` 在 shadow 阶段保持独立，Gateway 查询仅比较 Temporal Query 与投影并返回对账结果，不直接修复数据库。

离线对账同样不授予 TS 直接表权限。Core 使用 sqlc keyset 查询固定的 `dipole-agent/shadow` Run cohort，并通过 additive `ListTaskWorkflowProjectionSnapshots` 私有 RPC 返回 Task 与可空投影；TS 只负责 Temporal Query/Describe 比较和版本化报告。该读取接口不包含写入或修复语义。

`000002_conversation_sequence` 为历史消息按 `conversation_key + id` 回填连续序号，并创建 `conversation_sequences` 高水位表。新消息在 Message、Inbox 与 Outbox 的同一事务内锁定会话行并分配 `seq`；事务回滚会同时回滚高水位，旧 `before_id`/`after_id` 查询在兼容期继续保留。

`000003_read_and_device_checkpoints` 为 Conversation 投影回填 `last_message_seq/read_seq`，继续维护 `unread_count` 兼容字段，并增加独立 `device_sync_checkpoints`。已读操作只推进到调用方当时可见的 Seq；设备 checkpoint 通过显式 ACK 单调推进，超过当前用户 Inbox 最大 Seq 的请求会被拒绝。

`000004_message_search_index` 增加可重建的 MySQL 搜索投影基线。该基线已完成 Elasticsearch 迁移后从运行时退役，迁移表暂保留以支持历史环境和独立保留期评估；Search 的投影、回填和查询均以 Elasticsearch 为唯一运行实现。`MessageStore` 同时提供基于会话 Seq 的前后游标查询，旧 ID cursor 在兼容期继续保留。

`000005_hot_group_checkpoints` 从历史群 Timeline 回填 `group_sync_states`，并创建用户/设备/群级 `device_group_sync_checkpoints`。后续群消息在 Message、Inbox 与 Outbox 事务内 O(1) 推进群高水位；设备只在消息持久化完成后显式 ACK，低位 ACK 不会使 checkpoint 回退。

开发命令：

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
scripts/sqlc.sh generate
scripts/check-sqlc.sh
```

### D3：按风险逐仓储迁移

推荐顺序：

1. AICallLog、Admin、File 等低耦合查询。
2. User、Contact、Group、Conversation 等 Core metadata。
3. Outbox 与 Message 查询。
4. Message 写事务、Sync Inbox 和用户级序号锁。

每个 Repository 先增加 sqlc adapter，通过同一 contract test 后使用配置开关灰度。禁止一次性重写全部查询。

### D4：移除 GORM

- [x] 默认适配器切换为 sqlc，生产连接池、migration 与 Bloom 使用共享 `database/sql`。
- [x] 全部生产路径切到 sqlc 并完成兼容窗口验证。
- [x] 删除 GORM adapter、model tag、SQLite 方言测试和 `AutoMigrate`。
- [x] 移除 `gorm.io/*` 依赖，文档和脚本只保留 SQL migration 流程。
- [x] 保留最终 sqlc 功能契约、migration 回滚和并发事务证据。

## 5. 测试门禁

- Query unit：参数、nullable、排序、分页和错误映射。
- Repository contract：sqlc adapters 在隔离 MySQL fixture 上覆盖完整 Application Port 行为。
- Integration：真实 MySQL 覆盖事务、死锁重试、`FOR UPDATE`、Upsert 和唯一约束。
- Migration：空库升级、现有库升级、重复执行、单版本回滚和 drift 检查。
- Performance：关键消息写入、Inbox fanout 和历史查询对比基线。

## 6. 回滚

GORM 兼容回滚窗口已经关闭。后续 schema 变化继续遵循 expand/contract；应用回滚必须使用仍兼容当前 schema 的已发布版本，并记录 migration checkpoint，禁止依赖运行时 schema mutation。

## 7. 完成标准

- 生产代码与测试不再导入 `gorm.io/*`。
- 服务启动不执行 schema mutation。
- 所有 SQL 由 migration 或 `db/queries` 管理并进入代码审查。
- sqlc 生成结果可复现，Repository contract、MySQL 集成和迁移测试全部通过。
