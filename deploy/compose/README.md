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

Compose 文件中的相对路径以仓库根目录为基准。脚本调用应优先复用 `scripts/` 下的入口，以获得项目名、清理和回滚保护。
