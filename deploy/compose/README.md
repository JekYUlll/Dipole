# Compose 编排

本目录集中维护 Dipole 的集成、分发、集群和存储实验拓扑。默认本地开发入口仍为根目录的 `docker-compose.yml`，避免影响现有开发习惯。

## 文件

| 文件 | 用途 |
| --- | --- |
| `docker-compose.microservices.yml` | 微服务集成拓扑 |
| `docker-compose.dist.yml` | 分发镜像与回滚拓扑 |
| `docker-compose.cluster.yml` | Kafka、Cassandra 和可观测性集群演练 |
| `docker-compose.mysql-cluster.yml` | MySQL 高可用演练 |
| `docker-compose.redis-cluster.yml` | Redis Sentinel 故障转移演练 |
| `docker-compose.storage-lab.yml` | 隔离存储实验 |
| `../microservices/agent-active.yml` | 默认不加载的 Agent user-gray 只读 overlay |
| `../microservices/agent-interactive-active.yml` | 基于 user-gray 的显式审批交互消息候选 overlay |
| `../microservices/agent-memory-promotion.yml` | 基于 user-gray 的 reviewed Memory receipt 提交 overlay |
| `../microservices/agent-interactive-shadow.yml` | 隔离环境的只读 Agent Task 控制面 overlay |
| `../microservices/agent-deepseek-v4-flash-shadow.yml` | DeepSeek V4 Flash 的 JSON-text 与关闭 reasoning 兼容 overlay |
| `../microservices/remote-gpu-mysql-aio-compat.yml` | 仅限共享 Remote GPU 候选的 MySQL native AIO 兼容 overlay |

从仓库根目录执行，例如：

```bash
docker compose -f deploy/compose/docker-compose.microservices.yml config --quiet
docker compose -f deploy/compose/docker-compose.microservices.yml up -d --wait
```

启动远程开发拓扑前先运行主机门禁：

```bash
# 完整微服务、存储实验和负载测试
scripts/check-dev-host.sh remote-gpu

# 轻量 smoke 和低资源兼容性检查
scripts/check-dev-host.sh tencent-cloud
```

门禁只检查资源、Docker daemon 和 Compose 配置，不会创建容器或修改远程主机。共享环境部署必须额外提供真实密钥、独立 Compose project、版本绑定镜像和清理/回滚证据。

当共享 Remote GPU 已运行多个 MySQL 候选且 Linux AIO 配额不足时，可在独立候选项目中追加：

```bash
docker compose \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/remote-gpu-mysql-aio-compat.yml \
  up -d --wait
```

该 overlay 只为该候选 MySQL 关闭 native AIO，不影响基础拓扑或其他 Compose project；候选结束后通过同一 project 的 `down -v` 回滚其容器与卷。

Compose 文件中的相对路径以仓库根目录为基准。脚本调用应优先复用 `scripts/` 下的入口，以获得项目名、清理和回滚保护。

Agent Memory receipt 提交需要同时加载基础微服务、`agent-active.yml` 与 `agent-memory-promotion.yml`，并提供 `DIPOLE_AGENT_MEMORY_PROMOTION_AUTHORITY=operator_approved`。该 overlay 不作为默认开发路径，详细门禁与回滚见 [Agent Active 部署手册](../../docs/agent/AGENT-ACTIVE-DEPLOYMENT.md)。
