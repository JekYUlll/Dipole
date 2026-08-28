# C1 Go Realtime Baseline

本目录归档 C1 候选拓扑的首组 Go 实时数据面连接梯度证据。原始采集发生于 2026-08-28（Asia/Shanghai），所有应用节点使用同一干净提交和不可变镜像。

## Provenance

| Field | Value |
| --- | --- |
| Git revision | `a6f367fd67d79ace95c730388a5bd95ac70bcb1d` |
| Image ID | `sha256:138f8868b7282815acb9bf3fc709c0f53284c0b93c9bcc3f367ae2dd05b5984a` |
| Image source dirty | `false` |
| CPU | AMD Ryzen 7 8845H w/ Radeon 780M Graphics |
| Topology | isolated `dipole-c1`, `docker-compose.dist.yml` |
| Application endpoints | `18081..18083` |
| Agent runtime | `off` |

共享 `dipole-*` 拓扑在采集期间停止，候选拓扑使用独立 Compose project、端口、网段和 named volumes。候选 MySQL 由 one-shot migration 初始化。

## Workload

三档运行保持同一镜像、机器、Compose 配置和采集器参数。每个连接发送 2 条 direct message，WebSocket 接收窗口为 15 秒。

| Connections | Messages | Throughput | P50 | P95 | P99 | Peak lag | Summed CPU | Peak RSS | Voluntary CS | Involuntary CS |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 20 | 40 | 2.51 msg/s | 739 ms | 1072 ms | 1118 ms | 26 | 63.12% | 198.70 MiB | 550472 | 2251 |
| 50 | 100 | 4.14 msg/s | 2620 ms | 3925 ms | 4046 ms | 56 | 75.60% | 206.47 MiB | 755751 | 5627 |
| 100 | 200 | 4.85 msg/s | 5299 ms | 8084 ms | 8382 ms | 158 | 73.36% | 211.75 MiB | 1321350 | 7016 |

三档 acceptance、persistence 和 delivery rate 均为 100%，HTTP failure rate 为 0%，采样结束时 Kafka lag 均为 0。

## Interpretation

当前证据显示吞吐在 50 到 100 连接间趋于平台，同时端到端延迟、峰值 lag 和 context switch 明显上升，三节点合计 CPU 仍低于单个完整 CPU core。该现象提示业务串行化、外部等待、Conversation projection 或数据库路径可能构成主要限制，具体归因仍需分段 trace/profile 与故障恢复基线验证。

这组数据用于后续 Go/C++ 同条件对照，不预设 C++ 数据面能够消除端到端瓶颈。节点重启、Kafka 短暂不可用和恢复时间属于下一组独立证据。

## Evidence

每档均保留 operations、baseline JSON/Markdown、k6 summary、Kafka lag、Conversation Prometheus 快照、process samples/resources 和 runtime provenance。`SHA256SUMS` 覆盖全部原始证据；README 作为解释性索引不纳入该清单。

验证归档：

```bash
sha256sum --check benchmarks/c1-go-realtime-a6f367f-2026-08-28/SHA256SUMS
```
