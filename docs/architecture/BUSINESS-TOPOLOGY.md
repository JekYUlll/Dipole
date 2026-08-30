# 业务依赖拓扑与故障切换边界

Dipole 当前同时保留三种 Compose 拓扑。它们的验证范围必须分开记录，避免把基础设施故障演练误认为业务链路已经完成自动切换。

## 拓扑分层

| 拓扑 | Compose 入口 | Kafka | Redis | 适用范围 |
| --- | --- | --- | --- | --- |
| 单节点微服务 | `deploy/compose/docker-compose.microservices.yml` | 单 broker `kafka` | 单实例 `redis` | 本地开发、快速 smoke、回滚 |
| 基础设施集群 | `deploy/compose/docker-compose.cluster.yml` + `docker-compose.redis-cluster.yml` | 3 节点 KRaft | 3 Redis + 3 Sentinel | Kafka/Redis 组件级演练 |
| 业务集群 | 尚未提供 | 业务服务必须使用 broker 列表 | 业务服务必须使用 Sentinel 配置 | 业务故障切换与恢复证据 |

当前微服务 Compose 中的业务服务依赖名仍为 `kafka` 和 `redis`，并通过同一文件提供这两个单节点服务。单独启动集群 Compose，或把两个 Compose 文件直接拼接，均不能形成业务集群：前者没有业务容器，后者会产生服务名、依赖条件和网络边界冲突。

## 证据规则

组件级 smoke 可以证明：

- Kafka broker 副本、ISR、consumer rebalance 和 lag 行为；
- Redis Sentinel 选主、客户端重发现和可重建实时状态恢复。

只有业务集群演练才能证明：

- Core、Message、Sync、Search Indexer、Agent 和 Gateway 使用同一高可用依赖拓扑；
- broker 或 Redis 主节点故障期间，业务处理保留 offset、幂等和回滚语义；
- 业务服务恢复后，消息、Inbox、搜索投影和实时投递达到定义的收敛条件。

在业务集群入口落地前，报告必须将 Kafka/Redis 故障标记为“组件级”，不能写成“业务自动回切已验证”。

## 业务集群接入前置

后续新增业务集群 Compose 时必须同时满足：

1. 所有 Kafka consumer/publisher 使用至少两个可解析的 broker 地址，并将 topic replication、min ISR 和 `acks=all` 显式绑定到环境配置。
2. Redis 业务服务使用 Sentinel master discovery，不能只覆盖 `redis:6379` 单地址。
3. 服务 readiness 检查依赖真实的业务客户端连接，而不是仅检查基础设施容器健康状态。
4. 故障演练使用独立 Compose project、固定 revision、可回滚 receipt，并保存业务层消息、Inbox、lag 和投递结果。
5. 默认单节点路径保持可用，业务集群切换必须有显式开关和同版本回退路径。

## 当前结论

当前仓库已具备 Kafka 三节点和 Redis Sentinel 的组件能力，微服务默认路径仍是单节点。业务集群组合拓扑和自动业务回切证据属于后续 A6/C1 门禁，暂不宣称完成。
