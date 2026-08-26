# Dipole Sync Service

## 当前所有权

`dipole-sync` 是 A6 的第一个可回滚切片，负责以下持久同步查询：

- 按用户 `sync_seq` 读取 Durable Inbox。
- 每个 Sync Item 固化 `conversation_key + message_uuid + message_seq`，用于跨 MySQL/Cassandra 定位同一消息。
- 读取和推进设备 Cursor。
- 读取和推进热群 checkpoint。
- 通过受认证 Core Capability 校验群成员关系。
- 可选消费私聊/普通群 `message.created`，将事件时收件人快照物化为 Durable Inbox。

运行时固定初始化 `database/sql + sqlc SyncStore`、MySQL migration readiness、内部 gRPC 和 metrics，不初始化 Redis、完整 Core Repository 或 Message Service。仅当 `sync.projector_enabled=true` 时初始化 Kafka publisher/consumer；默认关闭时保持纯查询与 checkpoint 运行时。

## 过渡写入边界

当前 Message 事务继续原子提交 Message、普通群/私聊 Inbox 和 Transactional Outbox。这个边界在异步 Sync Projector 具备以下门禁前保持不变：

1. 消费者按消息 locator 和收件人幂等，并让同位置冲突进入重试/DLQ。
2. 固定高水位 Backfill 与事件重放能够恢复 Inbox。
3. 双写/影子比较覆盖离线、多设备、热群和提交乱序。
4. 故障恢复验证不会让设备 Cursor 永久跳过消息。

当前读取仍按 `message_uuid` 从 MySQL 批量补全完整 Message。locator 已独立持久化并通过 HTTP/gRPC 暴露，后续可以按相同 `(conversation_key, message_seq)` 从 Cassandra 影子补全并比较，而无需改变客户端 Cursor。

当前独立运行时已具备默认关闭的 Kafka Inbox 投影。Message 与 Projector 双运行时，正确 locator 重放保持 no-op；任一收件人发生 locator 冲突时，整批收件人事务回滚。旧私聊事件可从发送者/目标恢复收件人，旧群事件缺少事件时成员快照会失败并进入 Kafka 重试策略，避免使用当前成员关系猜测历史收件人。

热群事件携带 `sync_fanout=false`，Projector 不写用户 Inbox，客户端继续通过持久群高水位和 checkpoint 按 Conversation Timeline 拉取。

## 内部 RPC

- Sync listener/target 默认使用 `127.0.0.1:9094`。
- `dipole-sync` 调用 Core 时只允许 `GetGroupMember` 和 health check。
- Core 与 Gateway 可调用 Sync v1；protobuf `caller_service` 必须与传输层认证身份一致。
- 容器或远程地址必须启用 TLS 1.3 mTLS，明文只允许 loopback。

## 本地启动

先启动 Core Capability listener，再运行：

```bash
go run ./cmd/sync-service
```

配置至少需要启用 `internal_rpc.enabled`，提供共享凭据、Core target、Sync listen address 和可用的 MySQL schema。生产构建产物为 `dist/dipole-sync`。

Projector 双运行验证阶段额外设置：

```yaml
kafka:
  enabled: true

sync:
  projector_enabled: true
```

独立 consumer group 固定为 `dipole-sync-consumer`。启用前必须完成 migration v9，并先执行 `scripts/smoke-sync-projector.sh`。该 smoke 使用隔离的 Kafka 三节点与 MySQL 8.4，验证 Message 预写、Kafka 重复重放和热群跳过 fanout。

## 回滚

公开 HTTP 路由继续由 Core 提供，Inbox 写入路径保持不变。部署 `dipole-sync` 并验证 RPC 后，可将 Core 的 `sync.transport` 从默认 `local` 改为 `grpc`。独立服务异常时恢复 `local` 并重启 Core，进程内 SyncApplication 会立即接管现有 `/sync` 行为，无需回滚数据。

切流前可开启 `sync.shadow_queries=true`，异步比较 `List`、`GetCheckpoint` 和 `ListGroupCheckpoints` 的 Local/Remote 结果。Checkpoint advance 属于写操作，只会在 `sync.transport` 选定的主实现执行一次，不参与影子调用。关闭影子开关不会改变主链路响应。

Projector 异常时先恢复 `sync.projector_enabled=false` 并重启 `dipole-sync`。Message 事务仍持有 Inbox 写入责任，因此停用 Projector 不会形成新的同步数据缺口。固定快照 Backfill/Reconcile 和 consumer lag 门禁完成前，不得移除 Message 的 Inbox 写权限。
