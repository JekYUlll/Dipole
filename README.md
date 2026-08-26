Dipole

![logo](docs/images/dipole-logo.png)

[更新日志](CHANGELOG.md)

[架构债务台账](ARCHITECTURE-DEBT.md)

[平台演进计划](PLATFORM-EVOLUTION-PLAN.md)

[Agent Runtime 设计](AGENT-RUNTIME-DESIGN.md)

[GORM 到 sqlc 迁移计划](DATA-ACCESS-MIGRATION.md)

[Pencil 前端设计计划](FRONTEND-DESIGN-PLAN.md)

[IM Gateway 渐进部署手册](GATEWAY-DEPLOYMENT.md)

[最小微服务开发拓扑](MICROSERVICES-DEPLOYMENT.md)

## 数据库迁移

启动服务前先执行版本化 migration：

```bash
go run ./cmd/migrate -direction up
go run ./cmd/server
```

服务启动只校验 migration 版本，不修改 schema。baseline down 会删除业务表，只允许在一次性测试库中配合 `-allow-destructive` 使用。

sqlc 生成固定使用 `v1.31.1`：

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
scripts/sqlc.sh generate
scripts/check-sqlc.sh
```

生产数据访问统一使用 `database/sql + sqlc`，查询定义位于 `db/queries`，生成代码位于 `internal/data/mysql/generated`。

Kafka Envelope、schema version、重试和死信规则见 [Kafka 事件契约](KAFKA-EVENT-CONTRACT.md)。

`message.transport` 默认为 `local`；设为 `grpc` 后通过受认证网络 channel 调用独立 Message Service，关闭开关即可回切本地实现。

Message gRPC 契约生成固定使用 `protoc-gen-go v1.36.11` 和 `protoc-gen-go-grpc 1.6.2`：

```bash
scripts/check-proto.sh
```

独立 Message Service 的 mTLS、影子验证、流量切换与回滚步骤见 [MESSAGE-SERVICE-DEPLOYMENT.md](MESSAGE-SERVICE-DEPLOYMENT.md)。

可重复的 RPC 与后续端到端性能记录见 [PERFORMANCE-BASELINE.md](PERFORMANCE-BASELINE.md)。
