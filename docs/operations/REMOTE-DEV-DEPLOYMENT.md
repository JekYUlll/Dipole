# 远程开发部署与压测

本文仅用于开发环境，不承诺生产容量，也不替代共享环境发布审批。

Remote GPU 需要可用的 Docker Compose v2 插件（`docker compose version`）、Go 1.26+ 和 Git SSH；preflight 会将缺少插件报告为 `compose=plugin-missing`。Go 可通过 `DIPOLE_REMOTE_GO_ROOT` 指向用户态工具链；未指定时，远程入口会在 `/home/admin1/.local/go-*/bin/go` 中自动选择最高版本，依赖源可通过 `DIPOLE_REMOTE_GOPROXY` 指向受控缓存代理，避免修改系统 Go 和远端网络配置。只读 preflight 不安装系统组件，安装或升级应由主机管理员在维护窗口完成。

## 环境选择

- `remote-gpu`：用于完整微服务 Compose、存储实验、Agent Runtime 和负载/故障测试。
- `tencent-cloud`：用于轻量 smoke、API/WS 回归和低资源兼容性检查。
- `local`：用于单元测试、静态检查和构建前置验证；本机磁盘不足时禁止启动完整拓扑。

开始前执行：

```bash
scripts/check-dev-host.sh remote-gpu
```

日常部署、代码同步、镜像构建和完整压测统一从本机发起远端命令，但实际构建与运行都发生在 Remote GPU；本机不启动完整 Compose：

```bash
scripts/remote-dev.sh sync       # 仅同步已提交 revision
scripts/remote-dev.sh preflight  # 只读检查远端
scripts/remote-dev.sh test       # 远端 Go canonical 测试和静态门禁，要求 Go 1.26+
scripts/remote-dev.sh node-test  # 远端 Agent/Frontend Node 测试、类型检查和构建
scripts/remote-dev.sh multipart-smoke # 远端 MinIO Multipart 生命周期 smoke，不申请 GPU
scripts/remote-dev.sh build      # 远端构建候选镜像
scripts/remote-dev.sh bench      # 远端运行完整基准
scripts/remote-dev.sh down       # 仅停止本次 project
```

默认 `build` 只生成微服务镜像。完整 C1 三节点候选拓扑需要旧单体候选镜像时，显式开启附加构建；镜像标签绑定当前提交：

```bash
DIPOLE_REMOTE_BUILD_CANDIDATE=1 \
DIPOLE_REMOTE_BRANCH=master \
DIPOLE_REMOTE_GO_ROOT=/home/admin1/.local/go-1.27.0 \
scripts/remote-dev.sh build
```

脚本默认使用 SSH alias `LAB113-OPS`（用户 `admin1`）、远端目录 `/home/admin1/workspaces/Dipole` 和按用户隔离的 Compose project。`build`、`smoke-lite`、`bench` 会记录 GPU 进程快照并允许 CPU/容器型开发动作继续执行；活跃登录用户仍会默认阻断，只有取得明确维护窗口后才可设置 `DIPOLE_REMOTE_ALLOW_ACTIVE=1`。`test` 只执行远端测试和静态检查，不启动服务；脚本禁止隐式下载 Go toolchain，版本不足时快速失败。目录不存在时由 `sync` 在远端创建并通过 Git 获取提交。

`multipart-smoke` 只创建脚本自有的随机命名临时 MinIO 容器，使用 `GOTOOLCHAIN=local` 和 `DIPOLE_REMOTE_GO_ROOT` 提供的远端 Go 工具链；该动作不申请 GPU，也不经过活动 GPU 阻断，但仍要求脚本退出时完成容器清理。

`multipart-restart-smoke` 在相同隔离边界内上传首个分片，重启临时 MinIO 容器，再继续上传并完成对象，用于验证 Multipart 数据卷持久性。该动作不申请 GPU，不触碰其他容器或卷。

管理员已将 Go 1.27.0 以用户态方式放置于 `/home/admin1/.local/go-1.27.0`。远程入口未指定 `DIPOLE_REMOTE_GO_ROOT` 时会自动发现该工具链；需要固定版本时仍可显式指定：

```bash
DIPOLE_REMOTE_BRANCH=master \
DIPOLE_REMOTE_GO_ROOT=/home/admin1/.local/go-1.27.0 \
scripts/remote-dev.sh test
```

管理员已将 Node 22.12.0 以用户态方式放置于 `/home/admin1/.local/node-22.12.0`。`node-test` 会优先使用该路径；缺少依赖时执行 `npm ci --ignore-scripts`，随后以 `--package-lock=false` 补齐 optional dependencies，避免远端测试改写提交中的锁文件。测试前会拒绝已有的 `webapp` 脏状态，测试退出时仅清理本次构建产生的该目录变更：

```bash
DIPOLE_REMOTE_BRANCH=master \
DIPOLE_REMOTE_NODE_ROOT=/home/admin1/.local/node-22.12.0 \
scripts/remote-dev.sh node-test
```

## Remote GPU 流程

先确认主机没有活动实验需要避让，并使用独立目录、Compose project、端口段和网络。推荐使用提交绑定源码构建候选镜像：

```bash
export DIPOLE_ROOT=/home/admin1/workspaces/Dipole
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

实时数据面候选压测沿用 `scripts/bench/candidate_topology.sh`；Agent 默认保持 shadow 或 off，避免外部模型成本和延迟污染 IM 基线。完整 `k6` 基准和 Docker 构建固定在 Remote GPU 执行；本机仅保留脚本静态检查。

迁移任务确认已在远端可执行后，可先预览再停止本机 Dipole 负载：

```bash
scripts/drain-local-dipole.sh --dry-run
scripts/drain-local-dipole.sh --apply
```

该脚本只处理名称以 `dipole` 开头的运行中容器，保留容器定义、卷和镜像，不触碰其他 Compose project；需要恢复时使用原 Compose 配置重新启动对应 project。

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
验证最小集合使用：

```bash
COMPOSE_PROJECT_NAME=dipole-tencent-dev \
scripts/smoke-microservices-lite.sh
```

## 停止、证据与回滚

停止时只操作本次 project：

```bash
docker compose -p "${DIPOLE_PROJECT}" \
  -f deploy/compose/docker-compose.microservices.yml down --remove-orphans
```

压测记录必须包含提交和镜像摘要、配置摘要、主机资源快照、服务 readiness、P50/P95/P99、错误率、Kafka lag、磁盘和内存水位。发生 readiness 失败、数据不一致、错误率升高或资源越界时立即停止加压，回到上一提交或关闭本次 project；不要清理其他用户的容器、卷和进程。

当前 Remote GPU 若存在 GPU 任务，CPU/容器型开发部署可在隔离 project 下继续；若存在活动登录会话，仍需先取得明确维护窗口或显式批准。确需 GPU 的任务必须单独声明设备、显存预算和冲突检查。TencentCloud 凭据不得写入仓库、脚本或压测报告。
