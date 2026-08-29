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

## 写入所有权

默认 `message.inbox_write_mode=atomic` 时，Message 事务原子提交 Message、普通群/私聊 Inbox 和 Transactional Outbox。完成下列门禁后，可在独立 Message owner 设置 `projector`，让 Sync Projector 接管 Durable Inbox 新增写入：

1. 消费者按消息 locator 和收件人幂等，并让同位置冲突进入重试/DLQ。
2. 固定高水位 Backfill 与事件重放能够恢复 Inbox。
3. 双写/影子比较覆盖离线、多设备、热群和提交乱序。
4. 故障恢复验证不会让设备 Cursor 永久跳过消息。
5. `dipole_sync` 与 `dipole_message_projector` 最小权限启动探针通过，Message owner 的 Kafka 已启用并通过连接初始化。

`projector` 模式继续在同一事务提交 Message、Conversation Seq、群高水位与 Outbox，并跳过 `user_sync_inbox/user_sync_states`。模块化单体和其他无 Kafka 本地路径继续使用默认构造器，保持 atomic 行为。

当前主读取按 `message_uuid` 从 MySQL 批量补全完整 Message。开启 `sync.cassandra_shadow_hydration` 后，运行时按相同 `(conversation_key, message_seq)` 异步读取 Cassandra Timeline 并比较公开消息字段；客户端响应、错误语义和 Cursor 均继续取自 MySQL。比较任务最多并发 32 个，容量耗尽时记录 skipped，Cassandra 缺行、内容差异或读取错误均不改变主响应。

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
go run ./cmd/services/sync
```

配置至少需要启用 `internal_rpc.enabled`，提供共享凭据、Core target、Sync listen address 和可用的 MySQL schema。生产构建产物为 `dist/dipole-sync`。

Sync 可通过 `sync.mysql.*` 或 `DIPOLE_SYNC_MYSQL_*` 使用专用账号；空字段继承全局 MySQL 配置。迁移完成后应用 `configs/mysql/sync-service-grants.dist.sql`，并设置：

```yaml
sync:
  enforce_db_permissions: true
```

Message 切换 projector 权限时应用 `configs/mysql/message-service-projector-grants.dist.sql`，使用对应账号，并同时启用 `message.enforce_db_permissions=true`。该账号无 Inbox 权限，仅为旧 `/messages/offline` 保留 `groups/group_members` 的只读过滤能力；这两项临时授权随 AD-019 的客户端兼容周期结束后撤销。权限不满足职责边界时进程在开放 RPC 前失败。

Projector 双运行验证阶段额外设置：

```yaml
kafka:
  enabled: true

sync:
  projector_enabled: true
```

独立 consumer group 固定为 `dipole-sync-consumer`。启用前必须完成 migration v9，并先执行 `scripts/smoke-sync-projector.sh`。该 smoke 使用隔离的 Kafka 三节点与 MySQL 8.4，验证 Message 预写、Kafka 重复重放和热群跳过 fanout。

Cassandra hydration 双跑需要 Cassandra Timeline 已由 Projector/Backfill 补齐并通过 Reconcile，然后设置：

```yaml
cassandra:
  enabled: true

sync:
  cassandra_shadow_hydration: true
```

环境变量为 `DIPOLE_SYNC_CASSANDRA_SHADOW_HYDRATION=true`。开关启用但 `cassandra.enabled=false` 时，Sync Service 在开放 RPC 前拒绝启动。Prometheus 暴露 `dipole_sync_hydration_shadow_total{outcome}` 和 `dipole_sync_hydration_shadow_duration_seconds{outcome}`；`outcome` 包含 `match`、`mismatch`、`error` 和 `skipped`。

Cassandra-first primary hydration 使用 `DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION=true`，并同时设置 `DIPOLE_CASSANDRA_ENABLED=true` 与 `DIPOLE_CASSANDRA_HOSTS`。微服务 Compose 已提供这三个环境变量，默认分别为 `false`、`false` 和 `127.0.0.1:19042`；primary 与 shadow 互斥，Cassandra 失败会按同一 locator 整批回退 MySQL。切换前必须使用实际 Cassandra 网络地址、独立观测窗口和 AD-043 evidence gate，默认配置不自动启用。

隔离联调可以显式加载 `deploy/microservices/cassandra-primary.yml` 与 `--profile cassandra-primary`。该 profile 只启动 Cassandra 和一次性 schema init，并让 Sync 等待 init 成功后以 Cassandra-first 模式就绪。候选 smoke 额外将 Core 置于本地消息传输模式，绕开当前 Core/Message 远程初始化环，专门验证 Sync 的 Cassandra 主 hydration；默认微服务 Compose 不受影响。移除 profile 或关闭 `DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION` 即可回到 MySQL 主读，数据和设备 Cursor 不需要逆向迁移。

可重复执行以下 smoke 验证真实容器网络中的 profile、schema init 和 Sync readiness：

```bash
scripts/smoke-sync-cassandra-primary-compose.sh
```

Sync 新 consumer group 从 Kafka earliest retained offset 建立；已经提交过 offset 的 group 继续从自身 checkpoint 恢复。该语义关闭“Replay 固定快照后、consumer 首次建组前”的跳过窗口。Kafka retention 之前的消息由 Outbox Replay 覆盖，缺少 created Outbox 的早期行由历史 baseline 覆盖。

写责任切换观察窗按以下顺序执行：

1. 保持 Message 原子写 Inbox，启动 `sync.projector_enabled=true` 的 Sync Projector。
2. 执行固定 Outbox Replay 与历史 baseline Reconcile。
3. 等待 `dipole-sync-consumer` lag 归零，并确认 retry/DLQ 在约定观察窗没有新增。
4. 创建新的 Replay job 固化当前 Outbox 高水位，再次 Reconcile。
5. 应用 Sync/Message projector 最小授权并启用两侧权限门禁。
6. 将 `message.inbox_write_mode` 切为 `projector`，再次确认新消息最终进入 Inbox。

Prometheus 加载 `deploy/observability/sync-projector-alerts.yml`。`DipoleSyncProjectorLag` 持续两分钟触发 warning，retry 触发 warning，dead-letter 触发 critical；任一告警阻止写责任迁移。

## 回滚

公开 HTTP 路由继续由 Core 提供，Inbox 写入路径保持不变。部署 `dipole-sync` 并验证 RPC 后，可将 Core 的 `sync.transport` 从默认 `local` 改为 `grpc`。独立服务异常时恢复 `local` 并重启 Core，进程内 SyncApplication 会立即接管现有 `/sync` 行为，无需回滚数据。

切流前可开启 `sync.shadow_queries=true`，异步比较 `List`、`GetCheckpoint` 和 `ListGroupCheckpoints` 的 Local/Remote 结果。Checkpoint advance 属于写操作，只会在 `sync.transport` 选定的主实现执行一次，不参与影子调用。关闭影子开关不会改变主链路响应。

Cassandra hydration 发生 mismatch、error 或持续 skipped 时，设置 `sync.cassandra_shadow_hydration=false` 并滚动重启 Sync Service。该开关未承担主读职责，回滚不涉及 Cursor 或数据迁移。隔离双存储验收命令：

```bash
./scripts/smoke-sync-cassandra-hydration.sh
```

该 smoke 使用真实 MySQL 8.4 和 Cassandra 5.0.9 验证 match、payload mismatch、缺少投影和 MySQL 主结果隔离。

Projector 异常时先将 Message 账号恢复为 `configs/mysql/message-service-grants.dist.sql` 对应权限，将 `message.inbox_write_mode` 改回 `atomic` 并重启 Message owner。确认新消息已在事务内产生 Inbox 后，再停用 Sync Projector。故障窗口内的缺口通过固定 Outbox Replay/Reconcile 修复；历史缺口通过 baseline Restore 修复。

本地隔离验收：

```bash
./scripts/smoke-sync-write-ownership.sh
```

该 smoke 使用真实 MySQL 8.4 验证两类最小账号、Message+Outbox 无 Inbox 写入、Projector 收敛和 atomic 回退。

## 固定快照恢复与对账

先使用 migration v10 创建 `sync_replay_jobs`，再执行：

```bash
go run ./cmd/tools/sync-replay --job sync-inbox-20260827 --owner operator-a --batch-size 500
go run ./cmd/tools/sync-reconcile --job sync-inbox-20260827 --batch-size 500
```

Replay 只读取 `message.direct.created` 与 `message.group.created` Outbox，并在首次领取 job 时固化相关事件最大 ID。每批全部成功后才推进 checkpoint；进程中断、lease 过期或目标冲突时可以由新 owner 继续。热群事件计入处理水位，但按 `sync_fanout=false` 跳过用户 Inbox。

该快照覆盖具有 created Outbox 的消息。早期未产生 Outbox 的本地消息通过下述历史 baseline 保留原始 recipient/locator，禁止使用当前群成员关系猜测历史收件人。

Reconcile 要求对应 job 已完成，对快照内每个 Message UUID 精确比较预期收件人与实际 `user_uuid + conversation_key + message_seq`。一致时退出 0；缺行、额外收件人或 locator 冲突时输出有界 JSON 示例并退出 2。已完成 job 永久绑定原始高水位；需要再次修复时创建新 job 名，保留每次审计的快照边界。

本地隔离验收：

```bash
./scripts/smoke-sync-recovery.sh
```

Replay/Reconcile 已解决 Outbox-era Inbox 的固定快照恢复和数据差异门禁。Kafka lag、retry/DLQ 告警、投影 catch-up 窗口及 Message Inbox 写权限切换已通过独立真实环境演练。

## 历史 Inbox baseline

migration v11 增加不可变 baseline Job/Entry。切换写责任前，在 Message 原子写仍开启且业务写入已受控的维护窗口执行：

```bash
go run ./cmd/tools/sync-baseline --action capture --job sync-legacy-20260827
go run ./cmd/tools/sync-baseline --action reconcile --job sync-legacy-20260827
```

Capture 在 Repeatable Read 事务中固定当前 Inbox `sync_seq` 高水位，并归档所有找不到 created Outbox 的行。归档保留原始 `sync_seq + user_uuid + message_uuid + conversation_key + message_seq`，Manifest 记录 created Outbox 首尾 ID、行数和规范化 SHA-256。相同 Job 名重复 Capture 只校验并返回原归档，不移动边界。

Reconcile 扫描全部缺少 created Outbox 的当前 Inbox；快照后继续产生的 legacy 行会报告为 extra，原 recipient/locator 或 `sync_seq` 变化会报告为 conflicting，缺行报告为 missing。差异时输出有界 JSON 并退出 2。

仅 missing 状态允许自动恢复：

```bash
go run ./cmd/tools/sync-baseline --action restore --job sync-legacy-20260827
```

Restore 在单事务内以原 `sync_seq` 补齐缺失行。存在 extra/conflicting 时停止并退出 1，避免覆盖 Cursor 或收件人证据。隔离验收命令：

```bash
./scripts/smoke-sync-baseline.sh
```

写责任切换门禁需要同时满足历史 baseline Reconcile 与 Outbox-era Replay Reconcile；baseline 表在旧 Offline API 和 Message Inbox 回滚窗口结束前保留。
