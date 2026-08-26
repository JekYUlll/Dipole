# Cassandra Conversation Timeline

本文档记录 A3 第一切片的 Cassandra Message Store schema 与幂等写入契约。当前实现只参与隔离 contract，MySQL 继续承担生产消息写入和读取。

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
- Backfill、reconciliation 和 shadow-read 留在后续 A3 切片；新 consumer 从当前 Kafka 尾部开始，历史事实由 backfill 补齐。
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

## Verified Contract

真实 Cassandra 测试覆盖：

- 10,000 条消息的 bucket 边界。
- bucket 内按 `message_seq DESC` 返回。
- 首次 LWT 插入。
- 不同 event ID 的相同 payload 安全重放。
- 相同 Seq 的冲突 payload 拒绝覆盖。
- schema 文件按版本顺序加载。
- 独立 Kafka consumer group 对重复 created event 的端到端投影。

## References

- [Apache Cassandra data modeling](https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html)
- [Apache Cassandra lightweight transactions](https://cassandra.apache.org/doc/latest/cassandra/developing/cql/dml.html#lightweight-transactions)
- [Apache Cassandra GoCQL Driver](https://github.com/apache/cassandra-gocql-driver)
