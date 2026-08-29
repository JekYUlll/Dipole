# Realtime Benchmark Operations

本目录维护 Go 实时数据面的可复现基准、运行镜像来源证据和后续 C++ 对照入口。

## Candidate topology

候选拓扑复用 `deploy/compose/docker-compose.dist.yml`，同时固定以下隔离边界：

- Compose project 与容器前缀：`dipole-c1`
- 应用镜像：启动时解析为不可变 Docker image SHA-256
- 宿主端口：`18080/18443`、`18081..18083` 和独立基础设施端口
- Docker 网段：`10.201.0.0/24`
- Named volumes：归属于候选 Compose project
- Fresh MySQL：候选脚本以 one-shot `dipole-migrate` 命令完成迁移后再启动节点
- Agent：基准期间固定 `runtime_mode=off`，避免外部模型依赖污染实时数据面

默认 `deploy/compose/docker-compose.dist.yml` 仍使用 `dipole-*`、原宿主端口、`10.200.0.0/24` 和 `dipole-server:latest`。

候选镜像必须由干净提交构建，并与当前工作树 `HEAD` 相同：

```bash
IMAGE_TAG=c1-$(git rev-parse --short HEAD) scripts/docker-build.sh build
scripts/bench/candidate_topology.sh up "dipole-server:${IMAGE_TAG}"
```

脚本会验证 OCI revision、`io.dipole.source.dirty=false`，将 tag 解析为 image ID 后再启动。查看状态或回滚：

```bash
scripts/bench/candidate_topology.sh status
scripts/bench/candidate_topology.sh down
```

`down` 保留候选 named volumes，便于故障诊断和重启。清理数据属于单独的显式维护动作，脚本不会自动执行。

## Baseline run

候选拓扑就绪后，使用相同 Compose project 和候选端口采集：

```bash
COMPOSE_PROJECT_NAME=dipole-c1 \
COMPOSE_FILE=deploy/compose/docker-compose.dist.yml \
BASE_URL=http://127.0.0.1:18081 \
NODE1_WS=ws://127.0.0.1:18081 \
NODE2_WS=ws://127.0.0.1:18082 \
NODE1_HEALTH_URL=http://127.0.0.1:18081/health \
NODE2_HEALTH_URL=http://127.0.0.1:18082/health \
RUN_ID=c1-direct-50 \
SCENARIO=direct_msg \
SCENARIO_FILTER=direct_msg \
scripts/bench/run_bench.sh
```

`run_bench.sh` 在 k6 前验证健康端点、采集器工作树、每个服务的 container ID、image ID、revision、build time 和 dirty 状态。任一服务来源不一致都会停止采集；operations v4 同时记录实际 API/WS 端点。默认输出位于已忽略的 `scripts/bench/results/`，完成整组矩阵后再将选定原始证据归档到 `benchmarks/`。

连接梯度必须保持同一机器、Compose 文件、镜像、CPU/内存限制、采样周期和消息参数。每个报告至少保留 operations、baseline JSON/Markdown、k6 summary、Kafka lag、Conversation Prometheus 快照、process samples/resources 和 runtime provenance。

## Recovery drill

节点恢复使用独立证据契约，不放宽 steady-state baseline v4 对 PID 变化的拒绝规则。候选拓扑和同 revision 镜像就绪后执行：

```bash
COMPOSE_PROJECT_NAME=dipole-c1 \
COMPOSE_FILE=deploy/compose/docker-compose.dist.yml \
TARGET_SERVICE=dipole-node2 \
RUN_ID=c1-node2-recovery \
scripts/bench/recovery_drill.sh
```

脚本按以下顺序执行：

1. 记录目标节点的完整 container/image/revision/PID，并验证健康。
2. `stop` 目标节点，要求实际观察到健康不可用；EXIT trap 在后续失败时尝试恢复节点。
3. `start` 同一节点，等待 HTTP 健康和 Kafka consumer group 恢复到故障前成员数并连续稳定 5 秒，再记录新 PID 与恢复时间线。
4. 在恢复后的稳定进程上运行 baseline v4，要求完整来源证据、100% 接收/持久化/投递和 settled Kafka lag 为零。
5. 生成绑定精确 baseline SHA-256 的 `recovery-report.v1`。

consumer group 必须在故障前先达到稳定窗口。Kafka lag 解析会将“current offset 缺失且 log end 非零”的行保守计为积压，避免新 group 在 `LastOffset` 初始化窗口跳过记录后仍显示 lag 为零。正式证据应使用 fresh 或已确认无遗留未提交 offset 的候选 Kafka 卷。

该演练描述计划内单节点 stop/start 与恢复后链路。负载期间宕机、Kafka broker 故障、Redis Pub/Sub 切换和客户端重连补偿应分别采集，避免一个报告混合多个故障变量。
