# Kafka Cluster 开发与故障验收

本文档描述 A2 阶段的 Kafka 三节点 KRaft 基线。现有 `docker-compose.yml` 和 `docker-compose.microservices.yml` 继续提供单节点开发与微服务回滚路径；`docker-compose.cluster.yml` 当前用于隔离的基础设施集成和故障演练，后续逐步加入 MySQL、Redis、Cassandra 与 Elasticsearch profile。

## Durability Policy

集群基线固定为：

- 3 个 broker/controller，KRaft controller quorum 为 2。
- 业务 Topic 6 partitions、replication factor 3、`min.insync.replicas=2`。
- Producer 使用 `acks=all`；应用拒绝 `acks=none` 和 `min ISR > replication factor` 配置。
- Topic retention 显式配置，默认保留 168 小时。
- Broker 关闭 auto-create，Topic、`.retry` 与 `.dead` 仍由 Dipole Publisher 显式创建。

应用接入集群时使用：

```yaml
kafka:
  brokers:
    - kafka-1:9092
    - kafka-2:9092
    - kafka-3:9092
  topic_partitions: 6
  topic_replication_factor: 3
  topic_min_insync_replicas: 2
  topic_retention_hours: 168
  required_acks: all
```

Topic 配置只在首次创建时写入；已有 Topic 的 RF 迁移与配置 reconciliation 将作为独立可回滚步骤处理，不能依赖重启应用自动改写副本分配。

## Failure Smoke

执行：

```bash
./scripts/smoke-kafka-cluster.sh
```

脚本使用独立 Compose project 和 volumes，默认在退出时清理，依次验证：

1. 创建 RF=3、min ISR=2 的三分区 Topic。
2. 三 broker 健康时完成 `acks=all` 写入。
3. 停止一个 broker 后仍完成确认写入。
4. 再停止一个 broker，确认低于 min ISR 的写入无法 ACK。
5. 恢复第二个 broker，等待 ISR 回到 2 后重新写入。
6. 从头消费，确认三条已 ACK 消息完整，未 ACK 消息未进入日志。

调试时可以保留现场：

```bash
KEEP_STACK=1 ./scripts/smoke-kafka-cluster.sh
docker compose -p dipole-kafka-cluster-smoke -f docker-compose.cluster.yml down -v
```

## Remaining Gates

- 增加真实 Dipole Topic 的配置 drift/reconciliation 工具。
- 验证 Message/Core/Gateway consumer group 在 broker 故障与成员重启时的 rebalance。
- 暴露 under-replicated partitions、ISR、consumer lag、retry 和 DLQ 指标及告警。
