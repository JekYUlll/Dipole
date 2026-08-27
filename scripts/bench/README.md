# Realtime Benchmark Operations

本目录维护 Go 实时数据面的可复现基准、运行镜像来源证据和后续 C++ 对照入口。

## Candidate topology

候选拓扑复用 `docker-compose.dist.yml`，同时固定以下隔离边界：

- Compose project 与容器前缀：`dipole-c1`
- 应用镜像：启动时解析为不可变 Docker image SHA-256
- 宿主端口：`18080/18443`、`18081..18083` 和独立基础设施端口
- Docker 网段：`10.201.0.0/24`
- Named volumes：归属于候选 Compose project

默认 `docker-compose.dist.yml` 仍使用 `dipole-*`、原宿主端口、`10.200.0.0/24` 和 `dipole-server:latest`。

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
COMPOSE_FILE=docker-compose.dist.yml \
BASE_URL=http://127.0.0.1:18081 \
NODE1_WS=ws://127.0.0.1:18081 \
NODE2_WS=ws://127.0.0.1:18082 \
RUN_ID=c1-direct-50 \
SCENARIO=direct_msg \
SCENARIO_FILTER=direct_msg \
scripts/bench/run_bench.sh
```

`run_bench.sh` 在 k6 前验证采集器工作树、每个服务的 container ID、image ID、revision、build time 和 dirty 状态。任一服务来源不一致都会停止采集。默认输出位于已忽略的 `scripts/bench/results/`，完成整组矩阵后再将选定原始证据归档到 `benchmarks/`。

连接梯度必须保持同一机器、Compose 文件、镜像、CPU/内存限制、采样周期和消息参数。每个报告至少保留 operations、baseline JSON/Markdown、k6 summary、Kafka lag、Conversation Prometheus 快照、process samples/resources 和 runtime provenance。
