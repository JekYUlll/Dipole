# C1 Component Fault Evidence

## Kafka consumer rebalance

- 时间：2026-08-30
- 环境：Remote GPU，独立 Compose project `dipole-kafka-rebalance-*`
- 场景：三 broker、6 分区、复制因子 3、`min.insync.replicas=2`；两个 consumer member 各持有 3 个 partition。
- 操作：停止一个 consumer member，继续写入 20 条消息，等待剩余 member 接管全部 6 个 partition。
- 结果：consumer group 完成接管，lag 恢复为 `0`，脚本输出 `Kafka rebalance smoke passed`。

## Redis Sentinel failover

- 时间：2026-08-30
- 环境：Remote GPU，独立三 Redis 节点和三 Sentinel Compose project；GPU 任务保持运行。
- 场景：停止 Sentinel 当前 master，验证同一客户端重连、读写、Pub/Sub、Presence、热群状态和登录限流状态。
- 结果：约 4 秒完成主节点切换；客户端状态恢复，原 master 重新以 replica 加入，脚本输出 `Redis Sentinel failover smoke passed`。
- 备注：首次执行因远端未缓存 `alpine:3.21` 且 registry 拉取超时未进入探针；随后使用已缓存的 `alpine:3.22` 完成验证。探针镜像现已支持 `DIPOLE_REDIS_FAILOVER_PROBE_IMAGE` 配置。

## Evidence boundary

本文件记录 Kafka/Redis 组件级故障证据，不代表单 broker 候选拓扑或完整 IM 业务链路的自动切流已完成。候选拓扑中的 Kafka、Redis、背压和自动回切仍需独立验收。
