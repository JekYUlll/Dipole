# GORM 到 sqlc 迁移计划

本文档定义 Dipole 从 GORM 渐进迁移到 `database/sql + sqlc` 的边界、顺序、门禁和回滚方案。

## 1. 目标

- 使用版本化 SQL migration 维护数据库结构，停止依赖运行时 `AutoMigrate`。
- 使用 sqlc 生成 Go 侧类型安全查询，手写 SQL 成为可审查的数据访问契约。
- 保持 Service 和 Application Port 稳定，逐个 Repository 替换实现。
- 为 Go、TypeScript 和 C++ 服务共享 schema、事件与 RPC 契约。

多语言统一的锚点是版本化 SQL schema、查询语义、事件和服务 API。sqlc 负责生成 Go 数据访问代码；TypeScript 与 C++ 通过各自 driver 或 RPC 访问其拥有的数据，禁止多个服务跨语言直接共享同一业务表。

## 2. 当前基线

- GORM 相关代码覆盖 Store、11 个 Repository、migration、缓存适配和测试，共约 20 个 Go 文件。
- `internal/store/migrate.go` 在服务启动时对 12 个模型执行 `AutoMigrate`。
- Repository 包含事务、行锁、Upsert、分页、预加载和 Redis cache invalidation 等语义。
- Sync 并发安全依赖 MySQL `FOR UPDATE`，迁移时必须保留锁顺序和事务边界。

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
- [x] 保留 `AutoMigrate` 开关作为一个发布窗口内的紧急回退，默认关闭后再删除。

当前操作顺序：

```bash
go run ./cmd/migrate -direction up
go run ./cmd/server
```

应用默认只读校验 `schema_migrations` 版本，不在启动阶段修改 schema。baseline down 会删除全部业务表，仅允许在一次性测试库中显式使用 `-allow-destructive`。

### D2：建立 sqlc 基础设施

- [x] 固定 sqlc `v1.31.1` 配置、生成命令和生成代码检查策略。
- [x] 建立 `DBTX`、事务 helper、错误映射和真实 MySQL 测试 fixture。
- 为同一 Application Port 建立 GORM 与 sqlc contract test。
- [x] 提供生成漂移门禁，执行 `sqlc generate` 后检查工作区无差异。

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

- 全部生产路径切到 sqlc 后运行一段兼容窗口。
- 删除 GORM adapter、model tag、SQLite 方言测试和 `AutoMigrate`。
- 移除 `gorm.io/*` 依赖，文档和脚本只保留 SQL migration 流程。
- 保存最终 schema diff、性能对比和回滚演练证据。

## 5. 测试门禁

- Query unit：参数、nullable、排序、分页和错误映射。
- Repository contract：GORM/sqlc 在同一 fixture 上行为一致。
- Integration：真实 MySQL 覆盖事务、死锁重试、`FOR UPDATE`、Upsert 和唯一约束。
- Migration：空库升级、现有库升级、重复执行、单版本回滚和 drift 检查。
- Performance：关键消息写入、Inbox fanout 和历史查询对比基线。

## 6. 回滚

`data.mysql_adapter=gorm|sqlc` 只在迁移窗口存在。schema 变更遵循 expand/contract：先添加兼容结构，双版本代码稳定后再删除旧结构。Repository 切回 GORM 时不得要求数据回滚。

## 7. 完成标准

- 生产代码与测试不再导入 `gorm.io/*`。
- 服务启动不执行 schema mutation。
- 所有 SQL 由 migration 或 `db/queries` 管理并进入代码审查。
- sqlc 生成结果可复现，Repository contract、MySQL 集成和迁移测试全部通过。
