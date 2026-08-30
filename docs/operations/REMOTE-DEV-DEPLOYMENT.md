# 远程开发部署与压测

本文仅用于开发环境，不承诺生产容量，也不替代共享环境发布审批。

## 环境选择

- `remote-gpu`：用于完整微服务 Compose、存储实验、Agent Runtime 和负载/故障测试。
- `tencent-cloud`：用于轻量 smoke、API/WS 回归和低资源兼容性检查。
- `local`：用于单元测试、静态检查和构建前置验证；本机磁盘不足时禁止启动完整拓扑。

开始前执行：

```bash
scripts/check-dev-host.sh remote-gpu
```

## Remote GPU 流程

先确认主机没有活动实验需要避让，并使用独立目录、Compose project、端口段和网络。推荐使用提交绑定源码构建候选镜像：

```bash
export DIPOLE_ROOT=/data/zhangzhuyu/workspaces/Dipole
export DIPOLE_PROJECT=dipole-dev-<your-id>
export DIPOLE_INTERNAL_RPC_SHARED_SECRET='<development-only-secret>'

git clone git@github.com:JekYUlll/Dipole.git "${DIPOLE_ROOT}"
cd "${DIPOLE_ROOT}"
git checkout <commit>
scripts/check-dev-host.sh remote-gpu
IMAGE_TAG="dev-$(git rev-parse --short HEAD)" scripts/docker-build.sh build
```

完整微服务 smoke 使用独立 project，先验证配置，再启动并等待 readiness：

```bash
docker compose -p "${DIPOLE_PROJECT}" \
  -f deploy/compose/docker-compose.microservices.yml config --quiet
docker compose -p "${DIPOLE_PROJECT}" \
  -f deploy/compose/docker-compose.microservices.yml up -d --build --wait
scripts/smoke-microservices.sh
```

实时数据面候选压测沿用 `scripts/bench/candidate_topology.sh`；Agent 默认保持 shadow 或 off，避免外部模型成本和延迟污染 IM 基线。

## TencentCloud 流程

该主机只运行轻量 smoke。必须使用独立 project 和非默认宿主端口，并限制并发、消息体和测试时长：

```bash
scripts/check-dev-host.sh tencent-cloud
docker compose -p dipole-tencent-dev \
  -f deploy/compose/docker-compose.microservices.yml config --quiet
```

确认资源水位后再启动最小服务集合；禁止在该主机启用 Cassandra/Elasticsearch 全量实验、三节点压测或长时间 Agent 模型任务。

最小集合以 Gateway 为目标，让 Compose 只拉起它声明的依赖：

```bash
docker compose -p dipole-tencent-dev \
  -f deploy/compose/docker-compose.microservices.yml \
  up -d --wait gateway
```

不要使用不带服务名的 `up -d`，避免在 2 GiB 主机上同时启动 Agent 或后续新增的可选服务。

## 停止、证据与回滚

停止时只操作本次 project：

```bash
docker compose -p "${DIPOLE_PROJECT}" \
  -f deploy/compose/docker-compose.microservices.yml down --remove-orphans
```

压测记录必须包含提交和镜像摘要、配置摘要、主机资源快照、服务 readiness、P50/P95/P99、错误率、Kafka lag、磁盘和内存水位。发生 readiness 失败、数据不一致、错误率升高或资源越界时立即停止加压，回到上一提交或关闭本次 project；不要清理其他用户的容器、卷和进程。

当前 Remote GPU 已观察到多个活动登录会话和 GPU 任务，实际部署需要先取得明确维护窗口。TencentCloud 凭据不得写入仓库、脚本或压测报告。
