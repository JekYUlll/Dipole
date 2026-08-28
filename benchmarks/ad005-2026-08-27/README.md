# AD-005 Conversation Projection Baseline

本目录归档 2026-08-27 在候选提交 `2202f1f5dd022a27603656d32a205ed9939de9cf` 上采集的 Conversation State 写扩散证据。拓扑为三节点 `docker-compose.dist.yml`，CPU 为 AMD Ryzen 7 8845H。

| Scenario | Members | Measured messages | Conversation writes/message | Inbox writes/message | Delivery | P95 | Kafka settled lag |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Regular | 20 | 20 | 20 | 20 | 380/380 | 1924.15 ms | 0 |
| Hot | 20 | 20 + 1 warm-up | 20 | 0 | 380/380 | 668.80 ms | 0 |
| Regular | 100 | 10 | 100 | 100 | 990/990 | 10065.00 ms | 0 |
| Hot | 100 | 10 + 1 warm-up | 100 | 0 | 990/990 | 1853.00 ms | 0 |

`dipole_conversation_projection_writes_total` 在三个 Core 进程中分别计数成功 SQL upsert，采样器在运行前后求和取差值。`group_init` 单独记录，不进入 message write amplification。消息分母包含同一 run namespace 下的 warm-up，避免热群结果被低估。

证据确认 Conversation State 在普通群和热群中都保持 `O(group_size)` 写扩散。热群 notify + pull 将 Inbox 写扩散降为零，并明显降低本次链路延迟；现有数据无法把剩余延迟单独归因于 Conversation SQL，因此 AD-005 保持处理中，下一步需要按 projection 路径拆分数据库耗时后再评审读扩散或批处理。

这些结果用于同机、同提交、同拓扑的结构对照。共享机器负载会影响延迟，不能替代正式容量规划。

## Evidence Hashes

| File | SHA-256 |
| --- | --- |
| `group-regular-20.json` | `a85ee3c6b52e2505f1aae03ccfc0a995cd7c001d7b74b6907b5d4644837c0b50` |
| `group-hot-20.json` | `c42df3a29a4fd3865de51eca5f954bd908286fca3e80c1db8ecb611cc9d47a94` |
| `group-regular-100.json` | `2f342d767f204098b05fa91e9f18c83925642d1926b69c5a01ad3f4c9139ddc0` |
| `group-hot-100.json` | `a82da6de76708d9f22dafe3a501b92310bb20d3b9a13114785bfc3f74131a013` |

原始 operations、k6 summary、lag 样本和日志保留在被忽略的 `scripts/bench/results/`。
