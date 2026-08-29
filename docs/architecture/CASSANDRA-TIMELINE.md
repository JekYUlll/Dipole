# Cassandra Conversation Timeline

本文档记录 Cassandra Message Store 的 schema、实时投影、历史回填、对账、影子读取与 A4 灰度主读契约。MySQL 继续承担生产消息写入；显式 Seq cursor 可按会话 cohort 从 Cassandra 读取。

## Partition Model

```text
partition key: (conversation_key, bucket)
clustering key: message_seq DESC

bucket = (message_seq - 1) / 10000
```

边界示例：

| Seq | Bucket |
| ---: | ---: |
| 1 | 0 |
| 10,000 | 0 |
| 10,001 | 1 |
| 20,000 | 1 |

固定 Seq bucket 让相同会话与相同序号始终定位到同一分区，同时限制长会话的单分区增长。历史翻页从 cursor 所在 bucket 开始，耗尽后再进入前一个 bucket；跨 bucket 查询由后续 Cassandra read store 显式编排。

版本化 CQL 位于 `db/cassandra/001_timeline.cql`，当前实验 keyspace 使用 `NetworkTopologyStrategy` 和 `datacenter1: 1`。生产集群的 replication factor 必须由部署环境单独迁移，不能沿用单节点实验值。

## Projection Contract

`internal/platform/cassandra.TimelineStore.Append` 接收已经由 MySQL Transactional Outbox 和 Kafka 确认的 immutable created event，返回三类结果：

| 结果 | 条件 | Consumer 行为 |
| --- | --- | --- |
| Inserted | `(conversation_key, bucket, seq)` 首次出现 | 记录投影成功 |
| Duplicate | 主键已存在且 `payload_hash` 一致 | 安全 ACK 重放 |
| Conflict | 主键已存在且 `payload_hash` 不同 | 失败并进入重试/DLQ，不覆盖原记录 |

写入使用 `INSERT ... IF NOT EXISTS`。`payload_hash` 覆盖完整消息负载，排除 `event_id` 和 envelope version，因此同一事实被新 envelope 重放仍保持幂等。LWT 会增加写延迟；影子阶段优先保证冲突可检测，压测达到瓶颈后再评估普通幂等 upsert 与异步对账。

## Current Boundary

- 已引入 Apache Cassandra GoCQL Driver v2，并通过 Cassandra 5.0.9 真实 contract。
- Message bootstrap 中的独立 `cmd/tools/cassandra-projector` 使用专属 Kafka consumer group 消费 direct/group created event；默认配置关闭。
- Projector 启动只校验既有 schema，不执行自动建表，也不进入 Core 或 Gateway Composition Root。
- 独立 `cmd/tools/cassandra-backfill` 按固定 MySQL 高水位补齐历史数据，并使用持久 checkpoint 和 owner lease 支持失败恢复。
- 独立 `cmd/tools/cassandra-reconcile` 对已完成 Backfill 的固定快照执行数量、全量 hash、确定性内容样本和会话 Seq 连续性校验。
- 独立 Message Service 可执行 query-only Shadow-read，也可按稳定会话 cohort 服务显式 Seq cursor；消息写入、ID cursor 和 Offline Inbox 查询继续使用 MySQL。
- 当前表只支持 Conversation Timeline；按 UUID 查询、搜索和用户 Inbox 继续由现有存储负责。
- Schema 不由应用启动自动修改，后续 projector 接线前需要独立 migration owner。

## Projector Runtime

启用独立进程：

```yaml
cassandra:
  enabled: true
  hosts:
    - cassandra-1:9042
    - cassandra-2:9042
    - cassandra-3:9042
  keyspace: dipole_message_shadow
  local_datacenter: datacenter1
  timeline_bucket_size: 10000
  connect_timeout_seconds: 5
```

```bash
/app/dipole-cassandra-projector
```

执行 `scripts/smoke-cassandra-projector.sh` 可验证三节点 Kafka 到 Cassandra 的完整链路。脚本等待 consumer group 获得 partition assignment 后发布两次相同事件，并确认最终只有一条 Timeline 记录。

## History Backfill

建议按以下顺序部署：

1. 启动 Cassandra Projector，并确认专属 consumer group 已获得 partition assignment。
2. 执行 MySQL migration `000006_cassandra_backfill_jobs`。
3. 运行一次 Backfill；首次获取作业时固定 `messages.id` 高水位，后续消息由实时 Projector 处理。

```bash
/app/dipole-cassandra-backfill \
  --job message-timeline-v1 \
  --owner backfill-node-1 \
  --batch-size 500 \
  --lease-seconds 60
```

Backfill 按全局、不可变的 MySQL `messages.id` 扫描源数据，Cassandra 定位和排序仍使用会话内 `message_seq`。每批全部 Append 成功后才推进 `last_processed_id`；批内失败会安全重放已成功的 LWT 写入。`cassandra_backfill_jobs` 保存固定高水位、checkpoint、owner、lease、尝试次数和最近错误。

同一作业只允许一个有效 owner。进程异常退出后，下一 owner 在 lease 过期后接管；显式失败会立即释放 lease。完成的作业再次运行会返回 no-op，不扩大原高水位。执行 `scripts/smoke-cassandra-backfill.sh` 可验证失败不推进 checkpoint、恢复时 duplicate 幂等和最终完成状态。

## Reconciliation

对账只接受状态为 `completed` 且 checkpoint 等于高水位的 Backfill 作业。实时 Projector 可以继续处理新事件，对账范围始终停在该作业固定的 `source_high_watermark_id`。

```bash
/app/dipole-cassandra-reconcile \
  --job message-timeline-v1 \
  --batch-size 500 \
  --sample-modulus 100 \
  --max-examples 100 \
  > reconciliation.json
```

报告包含源消息数量、Cassandra 命中数、全量 payload hash 匹配数、缺失数、hash 差异数、确定性样本与会话 Seq 缺口。首条消息固定进入样本，其余消息由 Message UUID 的稳定 SHA-256 取模选择。差异样例只记录消息标识和 hash，不输出聊天正文。

退出码约定：

| 退出码 | 含义 |
| ---: | --- |
| 0 | 对账完成且报告一致 |
| 1 | 配置、MySQL、Cassandra 或执行错误，对账未完成 |
| 2 | 对账完成并确认存在数据差异 |

第一版按消息主键逐条读取 Cassandra，优先建立正确性门禁。达到大规模数据量后，再以测量结果决定是否增加分区批量读取和受控并发。

## Message Shadow-read

完成 Backfill 与 Reconciliation 后，可在独立 Message Service 上启用：

```yaml
cassandra:
  enabled: true

message:
  cassandra_shadow_reads: true
```

MessageStore 先执行 MySQL 查询并保存结果页快照，再按该页最小和最大 `message_seq` 异步读取 Cassandra。跨 bucket 范围由 TimelineStore 显式拆分，单次最多访问 64 个分区，结果统一按 Seq 升序比较。比较覆盖 UUID、ClientMessageID、路由目标、消息类型、正文、文件字段、过期时间和发送时间；MySQL 内部主键与维护时间不参与比较。

以下情况不会访问 Cassandra，并通过结构化日志记录 skip reason：

| Skip reason | 含义 |
| --- | --- |
| `primary_query_failed` | MySQL 主查询失败 |
| `empty_primary_page` | 主查询空页，无法从结果推导 Seq 区间 |
| `invalid_primary_sequence` | 页面含零值或无效 Seq |
| `shadow_capacity_exhausted` | 32 个异步比较槽位已占满，主动降载 |

Offline 查询属于 User Sync Timeline，单条消息/幂等查询和全部写操作也继续只访问 MySQL。影子查询错误或差异只写日志，不改变客户端结果。回滚只需关闭 `message.cassandra_shadow_reads` 并滚动重启 Message Service，不涉及数据迁移。

## Gradual Seq Read Routing

A4 灰度覆盖 Direct/Group `before_seq` 历史与 Group `after_seq` 增量补拉。MySQL `conversation_sequences` 提供轻量高水位，Cassandra 读取消息正文；`before_id`、`after_id`、Offline Inbox、单条查询和全部写操作继续访问 MySQL。

```yaml
cassandra:
  enabled: true

message:
  cassandra_shadow_reads: false
  cassandra_read_percentage: 10
  cassandra_read_verify_percentage: 100
```

会话键经过稳定 FNV-1a 哈希后进入 `0..99` cohort，同一会话在所有 Message 节点保持一致路由。`after_seq` 页必须完整覆盖 `(after_seq, min(high_watermark, after_seq+limit)]`；`before_seq` 的零值表示最新页，正值表示不包含上界，并读取最后 `limit` 个 Seq。两种方向都要求逐项连续；高水位读取失败、Cassandra 错误或缺行会使用原 Seq cursor 整页回退 MySQL。

新版 Web 历史首屏显式请求 `before_seq=0`，后续使用当前最旧正 `message_seq`，热群增量使用当前最大 `message_seq` 调用 `after_seq`。Cassandra 不保存 MySQL 内部自增 ID，客户端以 `message_id` 去重、以 `message_seq` 排序和分页；legacy ID cursor 不进入 Cassandra cohort。

`cassandra_read_verify_percentage` 对稳定的“会话 + before/after 操作”cohort 抽样。命中时，Cassandra 完整页会使用原 Seq cursor 同步读取 MySQL 并比较全部公开消息字段；match 继续返回 Cassandra，mismatch 记录 `payload_mismatch` 并整页返回 MySQL。核验侧 MySQL 暂时失败时记录 `mysql_error` 并返回已经通过 Seq 连续性校验的 Cassandra 页，避免核验依赖降低读取可用性。

`cassandra_read_percentage: 0` 是即时回滚开关，核验比例默认也为 0 且只能在主读比例大于 0 时启用。建议首次 1% 主读配合 100% 核验，确认稳定后逐步提高主读比例并降低核验比例。Prometheus 暴露 `dipole_message_read_route_total{route,fallback_reason}`、`dipole_message_read_route_duration_seconds{route}` 和 `dipole_message_read_verification_total{operation,outcome}`，其中 outcome 为 `match`、`mismatch` 或 `mysql_error`。

`deploy/observability/cassandra-read-alerts.yml` 定义三项停止门禁：任意 payload mismatch 为 critical；核验 MySQL error 为 warning；5 分钟 fallback 比例持续 2 分钟超过 5% 为 warning。任一 mismatch 立即将主读比例回切为 0；其余 warning 暂停提升比例并先诊断依赖与延迟。生产 Prometheus 必须抓取每个 Message Service 的 metrics endpoint；cluster profile 加载规则用于语法一致性，但其 Kafka-only 演练本身不提供 Message 样本。执行 `scripts/check-cassandra-read-alerts.sh` 可运行规则静态检查和固定时序 firing 测试。

## MySQL Body Retirement Gate

会话历史进入 Cassandra cohort 只证明 Seq Timeline 的读取能力。MySQL 完整消息当前还承担以下职责：

| 依赖 | 当前读取方式 | 退役前替代契约 |
| --- | --- | --- |
| Durable Inbox | `user_sync_inbox.message_uuid` 批量回查 `messages` | Inbox 固化 conversation/Seq/UUID，Sync Service 从 Message Store 补全并完成双跑比较 |
| Legacy Offline | 按全局 MySQL ID 扫描完整消息 | 客户端迁移到 Sync cursor，保留一个明确兼容窗口 |
| 幂等发送 | 按 sender/client ID 或 UUID 返回原完整消息 | 独立幂等结果快照或可按 UUID 定位的 Message Store 索引 |
| 文件授权 | 按 `file_id` 查询可访问消息 | 独立、可审计的文件消息授权元数据 |
| Search rebuild | MySQL 搜索投影或历史正文 | Elasticsearch 从 Cassandra 或归档事件全量重建 |
| Backfill/Reconcile | MySQL 固定高水位提供正文和 hash 基准 | 经校验的不可变备份与事件归档承担恢复源 |

因此 A4 保持 MySQL 完整消息写入。未来的 `message.mysql_write_mode=metadata_only` 只能在以下条件全部满足后实现并启用：

1. Cassandra 主读经过约定兼容窗口，fallback、mismatch 和延迟满足门限。
2. A5/A6 替代契约完成真实存储与端到端双跑，旧 Offline 使用率达到退出标准。
3. 固定快照备份可校验，Kafka/归档事件能够从 checkpoint 回放，并完成一次灾难恢复演练。
4. 切换记录包含 owner、开始时间、最后可回滚时间、数据 checkpoint 和逐步恢复命令。
5. metadata-only 节点与 full 节点滚动兼容；回滚窗口内仍保留可恢复的完整正文副本。

满足门禁后仍先灰度写模式，再观察新消息的 Sync、幂等、授权、搜索和恢复链路。历史 `messages` 表只会在回滚窗口与法定保留期结束后进入独立归档/删除决策。

## Verified Contract

真实 Cassandra 测试覆盖：

- 10,000 条消息的 bucket 边界。
- bucket 内按 `message_seq DESC` 返回。
- 首次 LWT 插入。
- 不同 event ID 的相同 payload 安全重放。
- 相同 Seq 的冲突 payload 拒绝覆盖。
- schema 文件按版本顺序加载。
- 独立 Kafka consumer group 对重复 created event 的端到端投影。
- MySQL 固定高水位、owner lease、失败释放、checkpoint 恢复和完成后 no-op。
- 真实 MySQL/Cassandra 故障恢复：失败批次保持 checkpoint，修复后重放 duplicate 并补齐 Timeline。
- 固定快照的数量、全量 hash、确定性字段样本与每会话 Seq 连续性报告。
- 人工篡改 Cassandra 后返回退出码 2，并提供有界、无正文的差异诊断。
- Query-only MessageStore 装饰器保持 MySQL 响应，异步比较 Cassandra 公开字段，并在容量耗尽时主动跳过。
- Cassandra 真实 contract 覆盖跨 bucket Seq 范围读取与升序合并。
- 真实 MySQL/Cassandra 路由同时覆盖 before/after Seq 完整页；人工删除 Timeline 行后，两种方向均整页回退 MySQL。
- 主读抽样核验覆盖公开字段 match、payload mismatch 和核验 MySQL 错误；真实篡改内容在 Seq 连续时仍会回退 MySQL。

## References

- [Apache Cassandra data modeling](https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html)
- [Apache Cassandra lightweight transactions](https://cassandra.apache.org/doc/latest/cassandra/developing/cql/dml.html#lightweight-transactions)
- [Apache Cassandra GoCQL Driver](https://github.com/apache/cassandra-gocql-driver)
