# Dipole Sync Service

## 当前所有权

`dipole-sync` 是 A6 的第一个可回滚切片，负责以下持久同步查询：

- 按用户 `sync_seq` 读取 Durable Inbox。
- 读取和推进设备 Cursor。
- 读取和推进热群 checkpoint。
- 通过受认证 Core Capability 校验群成员关系。

运行时只初始化 `database/sql + sqlc SyncStore`、MySQL migration readiness、内部 gRPC 和 metrics。它不初始化 Redis、Kafka、完整 Core Repository 或 Message Service。

## 过渡写入边界

当前 Message 事务继续原子提交 Message、普通群/私聊 Inbox 和 Transactional Outbox。这个边界在异步 Sync Projector 具备以下门禁前保持不变：

1. 消费者按事件 ID 和收件人幂等。
2. 固定高水位 Backfill 与事件重放能够恢复 Inbox。
3. 双写/影子比较覆盖离线、多设备、热群和提交乱序。
4. 故障恢复验证不会让设备 Cursor 永久跳过消息。

因此，当前独立运行时抽离的是查询与 checkpoint 所有权；Kafka 实时投影仍属于后续 A6 切片。

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

## 回滚

公开 HTTP 路由继续由 Core 提供，Inbox 写入路径保持不变。部署 `dipole-sync` 并验证 RPC 后，可将 Core 的 `sync.transport` 从默认 `local` 改为 `grpc`。独立服务异常时恢复 `local` 并重启 Core，进程内 SyncApplication 会立即接管现有 `/sync` 行为，无需回滚数据。

Checkpoint advance 属于写操作，当前不会执行影子双写。后续影子比较只覆盖 `List`、`GetCheckpoint` 和 `ListGroupCheckpoints` 等只读调用。
