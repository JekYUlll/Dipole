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

## 固定快照恢复与对账

先使用 migration v10 创建 `sync_replay_jobs`，再执行：

```bash
go run ./cmd/sync-replay --job sync-inbox-20260827 --owner operator-a --batch-size 500
go run ./cmd/sync-reconcile --job sync-inbox-20260827 --batch-size 500
```

Replay 只读取 `message.direct.created` 与 `message.group.created` Outbox，并在首次领取 job 时固化相关事件最大 ID。每批全部成功后才推进 checkpoint；进程中断、lease 过期或目标冲突时可以由新 owner 继续。热群事件计入处理水位，但按 `sync_fanout=false` 跳过用户 Inbox。

该快照覆盖具有 created Outbox 的消息。早期未产生 Outbox 的本地消息通过下述历史 baseline 保留原始 recipient/locator，禁止使用当前群成员关系猜测历史收件人。

Reconcile 要求对应 job 已完成，对快照内每个 Message UUID 精确比较预期收件人与实际 `user_uuid + conversation_key + message_seq`。一致时退出 0；缺行、额外收件人或 locator 冲突时输出有界 JSON 示例并退出 2。已完成 job 永久绑定原始高水位；需要再次修复时创建新 job 名，保留每次审计的快照边界。

本地隔离验收：

```bash
./scripts/smoke-sync-recovery.sh
```

Replay/Reconcile 已解决 Outbox-era Inbox 的固定快照恢复和数据差异门禁。Kafka lag、retry/DLQ 告警、投影 catch-up 窗口以及 Message 写权限退役仍需后续切片完成。

## 历史 Inbox baseline

migration v11 增加不可变 baseline Job/Entry。切换写责任前，在 Message 原子写仍开启且业务写入已受控的维护窗口执行：

```bash
go run ./cmd/sync-baseline --action capture --job sync-legacy-20260827
go run ./cmd/sync-baseline --action reconcile --job sync-legacy-20260827
```

Capture 在 Repeatable Read 事务中固定当前 Inbox `sync_seq` 高水位，并归档所有找不到 created Outbox 的行。归档保留原始 `sync_seq + user_uuid + message_uuid + conversation_key + message_seq`，Manifest 记录 created Outbox 首尾 ID、行数和规范化 SHA-256。相同 Job 名重复 Capture 只校验并返回原归档，不移动边界。

Reconcile 扫描全部缺少 created Outbox 的当前 Inbox；快照后继续产生的 legacy 行会报告为 extra，原 recipient/locator 或 `sync_seq` 变化会报告为 conflicting，缺行报告为 missing。差异时输出有界 JSON 并退出 2。

仅 missing 状态允许自动恢复：

```bash
go run ./cmd/sync-baseline --action restore --job sync-legacy-20260827
```

Restore 在单事务内以原 `sync_seq` 补齐缺失行。存在 extra/conflicting 时停止并退出 1，避免覆盖 Cursor 或收件人证据。隔离验收命令：

```bash
./scripts/smoke-sync-baseline.sh
```

写责任切换门禁需要同时满足历史 baseline Reconcile 与 Outbox-era Replay Reconcile；baseline 表在旧 Offline API 和 Message Inbox 回滚窗口结束前保留。
