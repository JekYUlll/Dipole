# MySQL Platform Infrastructure

本目录提供基于 `database/sql` 和 SQLC 的共享 MySQL 事务边界。

- `store.go` 负责数据库连接包装、SQLC Queries 和事务提交/回滚。
- SQLC 生成代码当前仍位于 `internal/data/mysql/generated/`，后续按服务拆分生成输出。
- `internal/data/mysql/store_compat.go` 只保留旧路径的类型别名和构造转发，禁止在旧目录恢复实现。
- 业务仓储应位于对应服务的 `internal/services/<service>/infrastructure/mysql/`。
