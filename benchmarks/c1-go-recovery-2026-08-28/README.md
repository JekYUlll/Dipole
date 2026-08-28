# C1 Go Node Recovery Baseline

本目录归档同日两次 node2 stop/start 演练。第一次由 baseline gate 拒绝，并促成 Kafka consumer readiness 与 lag 解析修复；第二次在 fresh project 中通过 recovery-report v1。

## Failed Attempt

目录：`failed-f657100/`

| Field | Value |
| --- | --- |
| Revision | `f6571004563b628cadf452abd6cf12582fcaa590` |
| Target | `dipole-node2` |
| PID | `846690 -> 849762` |
| HTTP unavailable to HTTP ready | 893 ms |
| Post-load attempted/accepted | 40 / 40 |
| Persisted/received | 0 / 0 |

HTTP 健康恢复后立即启动负载时，Kafka consumer group 尚未完成首次协调。Broker 在负载开始后约 10 秒才稳定到 72 members；默认 `LastOffset` 跳过了此前产生的 40 条 send-requested。旧 lag awk 忽略 `CURRENT-OFFSET=-` 行，因此失败 baseline 错误显示 settled lag 为 0；持久化与投递门禁仍正确阻断结果，未生成 passing recovery report。

## Passed Attempt

目录：`passed-ce4b600/`

| Field | Value |
| --- | --- |
| Revision | `ce4b6005650b5c29e1f01d3955ae3d0491559bba` |
| Image | `sha256:651fd9f80f1becb5582314fb646406940d7e744b5b27700d9a18f3de92c3d0a7` |
| Project | fresh `dipole-c1-recovery` |
| Target | `dipole-node2` |
| PID | `887973 -> 898410` |
| Stable consumer members | 72 before and after |
| Fault to unavailable | 650 ms |
| Start request to full ready | 13526 ms |
| Post-load attempted/persisted/received | 40 / 40 / 40 |
| Delivery | 100% |
| Kafka lag | peak 4, settled 0 |
| P50 / P95 / P99 | 993 / 1363.4 / 1413.79 ms |

`ready_observed_at` 同时要求 HTTP health 和 consumer group 恢复到故障前成员数并连续稳定 5 秒。恢复后 baseline v4 绑定同一 clean revision、三个运行镜像和新 PID；recovery report 再绑定 baseline SHA-256 `d4500d872bd390ed2fa9d66dcaab6006192851b18c18b6d84dbbdeef811507c9`。

该证据覆盖计划内单节点 stop/start 与恢复后负载。负载进行中的节点故障、Kafka broker 故障和 Redis Pub/Sub 切换仍需独立演练。

## Verification

`SHA256SUMS` 固定 failed/passed 两组原始证据，README 不纳入清单：

```bash
sha256sum --check benchmarks/c1-go-recovery-2026-08-28/SHA256SUMS
```
