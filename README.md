Dipole

![logo](docs/images/dipole-logo.png)

[更新日志](CHANGELOG.md)

[架构债务台账](ARCHITECTURE-DEBT.md)

[平台演进计划](PLATFORM-EVOLUTION-PLAN.md)

[Agent Runtime 设计](AGENT-RUNTIME-DESIGN.md)

[GORM 到 sqlc 迁移计划](DATA-ACCESS-MIGRATION.md)

[Pencil 前端设计计划](FRONTEND-DESIGN-PLAN.md)

## 数据库迁移

启动服务前先执行版本化 migration：

```bash
go run ./cmd/migrate -direction up
go run ./cmd/server
```

`mysql.auto_migrate` 默认关闭，仅在 GORM 到 sqlc 的兼容窗口内用于紧急回退。baseline down 会删除业务表，只允许在一次性测试库中配合 `-allow-destructive` 使用。

sqlc 生成固定使用 `v1.31.1`：

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
scripts/sqlc.sh generate
scripts/check-sqlc.sh
```

迁移窗口可通过 `DIPOLE_DATA_MYSQL_ADAPTER=gorm|sqlc` 选择数据适配器。当前 `sqlc` 只接管已经通过双适配契约测试的 Repository，设置为 `gorm` 可整体回切已迁移仓储。
