# MySQL Platform Infrastructure

本目录提供基于 `database/sql` 和 SQLC 的共享 MySQL 事务边界。

- `store.go` 负责数据库连接包装、SQLC Queries 和事务提交/回滚。
- `global.go` 提供 embedded/工具入口使用的 MySQL 全局连接初始化；生产启动入口和运维工具直接依赖本包，旧 `internal/store` 入口已退役。
- `migration/` 提供 schema migration runner，`config/` 提供 DSN 组装；它们属于 MySQL 平台基础设施。
- SQLC 生成代码当前仍位于 `internal/platform/mysql/generated/`，后续按服务拆分生成输出。
- `internal/data/mysql/store_compat.go` 和 `internal/data/mysql/repository/*_compat.go` 只保留旧路径的类型别名和构造转发，禁止在旧目录恢复实现。
- 业务仓储应位于对应服务的 `internal/services/<service>/infrastructure/mysql/`。
