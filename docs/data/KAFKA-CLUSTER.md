# Kafka Cluster 开发与故障验收

本文档描述 A2 阶段的 Kafka 三节点 KRaft 基线。现有 `docker-compose.yml` 和 `deploy/compose/docker-compose.microservices.yml` 继续提供单节点开发与微服务回滚路径；`deploy/compose/docker-compose.cluster.yml` 当前用于隔离的基础设施集成和故障演练，后续逐步加入 MySQL、Redis、Cassandra 与 Elasticsearch profile。

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
docker compose -p dipole-kafka-cluster-smoke -f deploy/compose/docker-compose.cluster.yml down -v
```

## Prometheus Gates

Cluster Compose 的 `observability` profile 包含 `kafka-exporter:v1.9.0` 与 `prometheus:v3.5.0`；旧 durability/rebalance smoke 继续只启动 broker。规则位于 `deploy/observability/kafka-alerts.yml`，覆盖：

- Dipole consumer group lag 持续高于零。
- Topic replica 数量与 ISR 数量之间存在缺口。
- retry Topic offset 或应用 retry 发布计数在五分钟窗口内增长。
- dead Topic offset 或应用 DLQ 发布计数在五分钟窗口内增长。

Core、Message 与 Gateway 可通过以下配置各自在独立端口暴露 Consumer 指标：

```yaml
metrics:
  enabled: true
  address: 0.0.0.0:9100
```

容器部署只允许 Prometheus 从私有服务网络访问该端口，不向宿主机或公网发布。裸机默认地址为 `127.0.0.1:9100`。应用指标包含 fetched、handled、committed、fetch/commit errors、retry/DLQ 发布累计值，以及 reader rebalance/error/queue 状态。

执行观测门禁：

```bash
./scripts/smoke-kafka-observability.sh
```

脚本校验 Prometheus 配置与规则，制造 consumer lag 和 retry/DLQ 流量，停止一个 broker 验证 ISR 缺口可见，再恢复 broker 并等待缺口归零。所有资源使用隔离 Compose project，并在退出时清理。

## Remaining Gates

- 增加真实 Dipole Topic 的配置 drift/reconciliation 工具。

## Consumer Rebalance

Dipole Consumer 显式使用 round-robin group balancer、同步 offset commit、3 秒 heartbeat、30 秒 session timeout 和 30 秒 rebalance timeout。成功处理或成功转移到 retry/DLQ 后才提交 offset；commit failure 会保留未提交位置并计入 snapshot。

执行消费组门禁：

```bash
./scripts/smoke-kafka-rebalance.sh
```

脚本创建 6-partition Topic 和两个同组 consumer，验证初始各分配 3 个 partition；终止一个 member 后，剩余 member 接管全部 6 个 partition，并在新增消息后把 group lag 重新降到 0。

`Consumer.CollectStats()` 提供累计 fetched/handled/committed、fetch/commit errors、retry/DLQ 发布数，以及 kafka-go reader 的周期增量 rebalances/errors/queue 指标。Prometheus Collector 是生产环境中的单一周期采集器，并将 reader delta 累加为单调 counter。
