# 最小微服务开发拓扑

`docker-compose.microservices.yml` 用于 M6 的本地开发与集成验收，部署以下最小服务集合：

```text
Client -> Gateway -> Core HTTP
          |    |
          |    +-> Core Capability gRPC
          +------> Message gRPC

Message -> MySQL / Redis / Kafka
Core    -> MySQL / Redis / Kafka / MinIO
Gateway -> Redis / Kafka
```

可选 `search` profile 追加：

```text
Gateway -> Search Service -> Core Capability
                         -> Elasticsearch read Alias
Kafka   -> Search Indexer -> Elasticsearch write Alias
```

该文件使用单节点基础设施和示例数据库凭据，适合本机验证；生产环境需要集群、Secret 管理、独立 Message 数据库账号、备份、监控和网络策略。

## 启动

先生成开发证书并构建统一镜像：

```bash
scripts/generate-internal-certs.sh
IMAGE_TAG=latest scripts/docker-build.sh build
```

设置强随机 RPC secret，再启动：

```bash
export DIPOLE_INTERNAL_RPC_SHARED_SECRET="$(openssl rand -hex 32)"
docker compose -f docker-compose.microservices.yml up -d --wait
```

公开入口为 `http://127.0.0.1:8080`。Core、Message、MySQL、Redis、Kafka 和 MinIO 只在 Compose 网络内可达。

三个应用进程共用一个镜像，通过 entrypoint 选择二进制。migration 作为一次性服务先执行；Core 与 Message 就绪后，Gateway 才开始接收流量。内部 gRPC 强制使用 TLS 1.3 mTLS，证书 CN 分别为 `dipole-core`、`dipole-message` 和 `dipole-gateway`。每个容器只挂载自己的证书、私钥与公共 CA 证书，CA 私钥保留在宿主机。

启用 `--profile search` 时，Search Indexer 先验收并初始化索引，随后 Search Service 以 `dipole-search` mTLS 身份连接 Core，并只读验收当前 Alias owner。内部链路就绪后以 `DIPOLE_SEARCH_ENABLED=true` 重建 Gateway，才会注册认证搜索路由；默认 false 保持原有反代行为。

## 自动验收

已有镜像时执行：

```bash
DIPOLE_IMAGE=dipole-server:latest scripts/smoke-microservices.sh
```

需要先构建镜像时执行：

```bash
BUILD_IMAGE=1 scripts/smoke-microservices.sh
```

脚本验证 Gateway health、Core HTTP 代理、mTLS 冷启动和 remote WS 唯一所有权，结束后自动删除隔离 project 与 volumes。设置 `KEEP_STACK=1` 可保留现场排障。

## 运行时依赖就绪

微服务 Compose 显式启用缓存型运行时依赖探针。默认每 5 秒探测一次，单次超时 1 秒；连续 3 次失败才退出就绪，连续 2 次成功才恢复，`/readyz` 本身只读取缓存状态。带 gRPC listener 的服务会把相同状态同步到标准 gRPC health，关闭时先进入 not-ready 再排空 RPC。

| 服务 | 纳入 readiness 的关键依赖 | 保持在专项指标或降级路径的依赖 |
| --- | --- | --- |
| Core | MySQL | Redis、Kafka Outbox、MinIO |
| Message owner | MySQL、Core RPC、Kafka | Redis cache、Cassandra shadow/fallback |
| Sync | MySQL、Core RPC | Kafka projector、Cassandra shadow hydration |
| Gateway | Redis、Core RPC、Message RPC、Kafka | Search、Agent control |
| Search | Elasticsearch、Core RPC | 无 |
| Search Indexer | Elasticsearch、Kafka | 无 |
| Cassandra Projector | Cassandra、Kafka | 无 |

配置项位于 `metrics.dependency_*`。普通配置默认关闭动态探针，便于模块化单体和测试保持兼容；微服务 Compose 通过环境变量统一启用。`dipole_dependency_ready{service,dependency}` 提供当前滞回结果，`dipole_dependency_probe_failures_total{service,dependency}` 累计探测失败，并由 `DipoleCriticalDependencyNotReady` 报警定位具体依赖。

使用以下演练验证 Elasticsearch 隔离期间只有 Search 与 Search Indexer 摘流，Core、Message、Sync、Gateway 继续 ready，所有应用容器 ID 保持不变；恢复后 Search 两个进程重新进入 ready：

```bash
BUILD_IMAGE=1 scripts/smoke-runtime-dependency-readiness.sh
```

该演练只停止并重启隔离 project 内的 Elasticsearch，不重启应用服务。设置 `KEEP_STACK=1` 可保留现场。

## 回滚

代码层继续保留 `gateway.mode=embedded` 与 `message.transport=local`。容器拓扑故障时按 [Gateway 部署手册](GATEWAY-DEPLOYMENT.md) 和 [Message 部署手册](MESSAGE-SERVICE-DEPLOYMENT.md) 回切；schema 与消息数据无需逆向迁移。
