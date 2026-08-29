# 微服务镜像隔离

Dipole 的 Go 服务入口已经按部署边界拆分到 `cmd/services/`。当前过渡部署仍使用包含多个二进制的共享镜像，便于保持旧 Compose 和回滚路径稳定。

## 候选镜像

`deploy/images/go-service.Dockerfile` 是单服务镜像模板。每次构建只复制一个指定二进制到 `/app/service`，镜像不携带其他服务或一次性工具。`migrate` 与六个长期服务都使用同一模板，保证 schema 迁移和服务代码来自同一构建基线。`isolated-images.yml` 同时覆盖旧 Compose entrypoint，避免共享镜像的二进制路径泄漏到候选部署。

```bash
scripts/docker-build.sh backend
scripts/docker-build-microservice-images.sh
```

共享环境候选验证使用独立 Compose project 和 Gateway 端口，默认自动清理自己的容器、卷和临时证书；默认 smoke 覆盖生产核心路径，Search profile 由静态 Compose 门禁和独立 Search 验证覆盖：

```bash
BUILD_IMAGE=1 GATEWAY_PORT=18080 scripts/smoke-microservice-isolated-images.sh
```

需要验证 Search 与 Search Indexer 候选镜像时显式启用 profile：

```bash
SMOKE_SEARCH_PROFILE=1 GATEWAY_PORT=18080 scripts/smoke-microservice-isolated-images.sh
```

需要验收候选 Gateway 的真实消息写入、Inbox 和 Seq Timeline 读取时，额外启用消息链路检查：

```bash
SMOKE_MESSAGE_FLOW=1 GATEWAY_PORT=18080 scripts/smoke-microservice-isolated-images.sh
```

需要验证候选镜像的 Kafka assignment 和 Search 依赖故障恢复时：

```bash
ISOLATED_IMAGES=1 scripts/smoke-runtime-dependency-readiness.sh
```

可以通过环境变量覆盖镜像标签：

```bash
DIPOLE_CORE_IMAGE=registry.example/dipole-core:candidate \
DIPOLE_GATEWAY_IMAGE=registry.example/dipole-gateway:candidate \
scripts/docker-build-microservice-images.sh
```

构建脚本会为每个镜像写入 Git revision、构建时间和 dirty provenance。它只构建镜像，不修改 Compose、数据库、Kafka consumer group 或 authority。

## 切换与回滚

当前 `docker-compose.microservices.yml` 继续使用 `DIPOLE_IMAGE` 共享镜像。验证候选镜像时，以 Compose override 将对应服务的 `image` 替换为单服务标签，并保留原变量作为回滚值：

```bash
docker compose \
  -f docker-compose.microservices.yml \
  -f deploy/microservices/isolated-images.yml \
  config --quiet
```

若候选服务未通过 readiness、RPC、消息写入或 Seq Timeline 读取 smoke，移除 override 即回到共享镜像；该过程不需要回滚 schema、offset 或 authority。

## 门禁

- 每个候选镜像必须只包含 `/app/service` 及运行时证书/时区文件。
- 构建前必须存在对应 `dist/dipole-*` 二进制。
- 生产切换前仍需通过 `scripts/smoke-microservice-isolated-images.sh`、`scripts/smoke-microservices.sh` 和依赖 readiness 检查。
- 本切片不改变默认 Go authority、Kafka topic、数据库权限或 Agent 开关。
