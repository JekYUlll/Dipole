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

[Search Service 渐进部署手册](SEARCH-SERVICE-DEPLOYMENT.md)

## 数据库迁移

启动服务前先执行版本化 migration：

```bash
go run ./cmd/migrate -direction up
go run ./cmd/server
```

服务启动只校验 migration 版本，不修改 schema。baseline down 会删除业务表，只允许在一次性测试库中配合 `-allow-destructive` 使用。

多语言仓库的 Go 全量门禁使用：

```bash
LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu scripts/check-go.sh
```

该脚本覆盖 `cmd`、`db`、`docs/swagger` 和 `internal` 下的全部 Go package，并依次执行 test 与 vet。Temporal npm 包携带上游 SDK 源树，安装 `agent-runtime/node_modules` 后不再使用根级 `go test ./...`，避免把第三方源码误识别为本项目 package。

sqlc 生成固定使用 `v1.31.1`：

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
scripts/sqlc.sh generate
scripts/check-sqlc.sh
```

生产数据访问统一使用 `database/sql + sqlc`，查询定义位于 `db/queries`，生成代码位于 `internal/data/mysql/generated`。

前端工具链固定使用 Node.js 22.12+ LTS、Vite 8 和 Vitest 4：

```bash
cd frontend
nvm use
npm ci
npm run test:toolchain
npm test
npm run build
```

`test:toolchain` 验证 `/app/` 静态资源基路径、生产输出边界及 HTTP/WebSocket 开发代理。代理默认目标为 `http://localhost:80`，隔离验收可通过 `DIPOLE_WEB_PROXY_TARGET` 覆盖。

Kafka Envelope、schema version、重试和死信规则见 [Kafka 事件契约](KAFKA-EVENT-CONTRACT.md)。
Kafka 三节点开发基线与故障验收见 [Kafka Cluster 文档](KAFKA-CLUSTER.md)。
MySQL InnoDB Cluster、Router writer 路由与主切换验收见 [MySQL Cluster 文档](MYSQL-CLUSTER.md)。
Redis Sentinel、实时状态语义与故障验收见 [Redis Cluster 文档](REDIS-CLUSTER.md)。
Cassandra 与 Elasticsearch 隔离实验及影子投影边界见 [Storage Lab 文档](STORAGE-LAB.md)。
Cassandra 会话 Timeline 分区与影子写入契约见 [Cassandra Timeline 文档](CASSANDRA-TIMELINE.md)。
Elasticsearch 版本化索引、Alias 与幂等投影契约见 [Message Search 文档](ELASTICSEARCH-SEARCH.md)。

`message.transport` 默认为 `local`；设为 `grpc` 后通过受认证网络 channel 调用独立 Message Service，关闭开关即可回切本地实现。

Message gRPC 契约生成固定使用 `protoc-gen-go v1.36.11` 和 `protoc-gen-go-grpc 1.6.2`：

```bash
scripts/check-proto.sh
```

独立 Message Service 的 mTLS、影子验证、流量切换与回滚步骤见 [MESSAGE-SERVICE-DEPLOYMENT.md](MESSAGE-SERVICE-DEPLOYMENT.md)。

可重复的 RPC 与后续端到端性能记录见 [PERFORMANCE-BASELINE.md](PERFORMANCE-BASELINE.md)。
