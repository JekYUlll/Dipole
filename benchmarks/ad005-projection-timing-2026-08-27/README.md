# AD-005 Conversation Projection Timing

本目录归档 2026-08-27 在候选提交 `4343684011a02112eb3e9233e7c4279bf64a4ee9` 上采集的 Conversation State Repository 调用耗时证据。拓扑为三节点 `docker-compose.dist.yml`，CPU 为 AMD Ryzen 7 8845H。

| Scenario | Members | Measured messages | Group-message calls | Average call | P95 bucket upper bound | Inbox writes/message | Delivery | End-to-end P95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Regular | 20 | 20 | 400 | 14.07 ms | 50 ms | 20 | 380/380 | 693.45 ms |
| Hot | 20 | 20 + 1 warm-up | 420 | 12.43 ms | 25 ms | 0 | 380/380 | 329.60 ms |
| Regular | 100 | 10 | 1000 | 23.07 ms | 50 ms | 100 | 990/990 | 8189.00 ms |
| Hot | 100 | 10 + 1 warm-up | 1100 | 21.97 ms | 50 ms | 0 | 990/990 | 1346.00 ms |

四组的 Conversation projection 错误均为 0，成功 Histogram 次数与成功 Counter 完全一致，Kafka settled lag 均为 0。P95 是 Prometheus Histogram 的桶上界，表达该分位落入的最大边界，不能当作精确分位值。

普通与热群的单次 Conversation SQL 边界耗时接近，而同规模端到端 P95 差异显著，支持 Inbox/投递策略是模式间差异的重要来源。Conversation 仍按成员串行写入：完整 workload 的 group-message Repository 调用累计耗时为 5.22 至 24.17 秒，`O(group_size)` 成本仍需在更大规模或候选批处理/读扩散方案中治理。

100 人场景使用 22 秒 receiver connection window，20 人场景使用默认 15 秒；两者均低于 k6 的 25 秒场景上限。此前两次 100 人普通群的 15 秒样本只观察到 792/990 与 693/990 receipt，基准门禁拒绝且未纳入本目录。提高窗口用于完整观察已确认链路，没有降低 delivery gate。

每个场景均保存：

- `*.k6-summary.json`：客户端原始摘要；
- `*.operations.json`：operations v3 环境、参数、存储与计时证据；
- `*.conversation-metrics.json`：逐节点快照差分后的派生结果；
- `*.conversation-dipole-node{1,2,3}.{before,after}.prom`：原始 Prometheus 快照；
- `*.lag`：Kafka lag 采样；
- `*.baseline.json`：baseline v3 规范化报告。

`SHA256SUMS` 覆盖上述全部证据文件。节点在采集后已恢复默认热群阈值 `200/50`。
