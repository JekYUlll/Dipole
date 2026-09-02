# MySQL Cluster 开发与故障验收

本文档描述 A2 阶段的 MySQL 8.4 InnoDB Cluster 基线。现有 `docker-compose.yml` 与 `deploy/compose/docker-compose.microservices.yml` 继续提供单节点开发和快速回切；`deploy/compose/docker-compose.mysql-cluster.yml` 仅用于隔离的 HA 集成与故障演练。

## Topology

```text
Dipole database/sql pool
          │
          ▼
MySQL Router :6446 (read-write)
          │
          ▼
InnoDB Cluster / single-primary
  mysql-1  mysql-2  mysql-3
```

- 三个固定为 MySQL `8.4.8` 的成员启用 GTID、ROW binlog 和 Group Replication 所需配置；Router 固定为 `8.4.10`。
- `mysqlsh` AdminAPI 建立 single-primary InnoDB Cluster，并通过 clone provisioning 加入另外两个成员。
- MySQL Router 从 Cluster metadata bootstrap，`6446` 只路由到当前 PRIMARY。
- Core、Message、migration 和所有 sqlc Repository 继续使用同一个 writer endpoint。
- 当前不启用 read/write splitting；Conversation、Outbox、Sync Seq 与大量查询依赖 read-your-writes 和事务锁语义，副本读取需要独立一致性设计。

Cluster Compose 固定发布 `127.0.0.1:16446 -> Router:6446`。接入该环境时配置：

```yaml
mysql:
  host: 127.0.0.1
  port: 16446
```

生产部署应在每个应用节点旁部署 Router，避免单个 Router 成为共享故障点；本地 profile 的单 Router 专门用于验证数据库成员切换。

## Migration Ownership

`cmd/tools/migrate` 仍是唯一 schema 变更入口。Runner 的 Up/Down 使用 schema-scoped MySQL advisory lock：

```text
dipole:migrate:<sha256(database-name)[:48]>
```

锁绑定独立数据库连接，等待上限 30 秒。多个 migration 进程同时启动时串行执行，后进入者在取得锁后重新读取 ledger。应用 Runtime 只执行 `ValidateCurrent`，不申请 migration lock，也不修改 schema。

## Failure Smoke

执行：

```bash
./scripts/smoke-mysql-cluster.sh
```

脚本使用独立 Compose project 和 volumes，并依次验证：

1. 从空卷配置三个成员、创建 Cluster、clone 加入副本并 bootstrap Router。
2. 通过 Router writer endpoint 执行全部版本化 migration。
3. 使用最小权限 `dipole` 账号建立长期 `database/sql` 连接池并提交切换前记录。
4. 停止当前 PRIMARY，等待 Group Replication 选举和 Router metadata 刷新。
5. 复用同一连接池连接新 PRIMARY，提交切换后记录，并确认切换前已提交记录仍可见。
6. 重启旧 PRIMARY，通过 AdminAPI `rejoinInstance` 恢复成员并等待 `ONLINE`。

调试时可保留现场：

```bash
KEEP_STACK=1 ./scripts/smoke-mysql-cluster.sh
```

保留现场后根据脚本输出的 project name 手工执行 `docker compose ... down --volumes`。该 Compose 使用固定开发凭据，不可直接用于生产。

## Failure Semantics

- PRIMARY 进程停止时，现有 server connection 会断开；`database/sql` 丢弃坏连接并重新连接 Router。
- failover 窗口内请求允许返回短暂数据库错误，由上层幂等键和客户端重试控制重复写入。
- 已提交事务必须在新 PRIMARY 可见；未确认事务由调用方按幂等键重试。
- quorum 丢失时 Router 不接受写流量，禁止通过强制写入单节点绕过 Group Replication 一致性。
- 节点重启与节点 rejoin 属于运维恢复动作，不影响 Router 已切换到新 PRIMARY 的业务可用性。
