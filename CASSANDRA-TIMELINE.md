# Cassandra Conversation Timeline

本文档记录 A3 Cassandra Message Store 的 schema、实时投影与历史回填契约。当前 Cassandra 仍是影子存储，MySQL 继续承担生产消息写入和客户端读取。

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

`internal/data/cassandra.TimelineStore.Append` 接收已经由 MySQL Transactional Outbox 和 Kafka 确认的 immutable created event，返回三类结果：

| 结果 | 条件 | Consumer 行为 |
| --- | --- | --- |
| Inserted | `(conversation_key, bucket, seq)` 首次出现 | 记录投影成功 |
| Duplicate | 主键已存在且 `payload_hash` 一致 | 安全 ACK 重放 |
| Conflict | 主键已存在且 `payload_hash` 不同 | 失败并进入重试/DLQ，不覆盖原记录 |

写入使用 `INSERT ... IF NOT EXISTS`。`payload_hash` 覆盖完整消息负载，排除 `event_id` 和 envelope version，因此同一事实被新 envelope 重放仍保持幂等。LWT 会增加写延迟；影子阶段优先保证冲突可检测，压测达到瓶颈后再评估普通幂等 upsert 与异步对账。

## Current Boundary

- 已引入 Apache Cassandra GoCQL Driver v2，并通过 Cassandra 5.0.9 真实 contract。
- 独立 `cmd/cassandra-projector` 使用专属 Kafka consumer group 消费 direct/group created event；默认配置关闭。
- Projector 启动只校验既有 schema，不执行自动建表，也不进入 Core、Message 或 Gateway Composition Root。
- 独立 `cmd/cassandra-backfill` 按固定 MySQL 高水位补齐历史数据，并使用持久 checkpoint 和 owner lease 支持失败恢复。
- 独立 `cmd/cassandra-reconcile` 对已完成 Backfill 的固定快照执行数量、全量 hash、确定性内容样本和会话 Seq 连续性校验。
- Shadow-read 留在后续 A3 切片；客户端与 Message Service 当前不读取 Cassandra。
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

## References

- [Apache Cassandra data modeling](https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html)
- [Apache Cassandra lightweight transactions](https://cassandra.apache.org/doc/latest/cassandra/developing/cql/dml.html#lightweight-transactions)
- [Apache Cassandra GoCQL Driver](https://github.com/apache/cassandra-gocql-driver)
