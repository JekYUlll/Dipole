# Dipole Performance Baseline

Run ID: `g0-group-hot-20260827-final`

Scenario: `group_hot`

Captured at: `20260827T013945Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `ec979d4e237ccf7d0158bf8ce01c96e896118a93` |
| CPU | AMD Ryzen 7 8845H w/ Radeon 780M Graphics |
| Topology | `docker-compose.dist.yml` |
| Benchmark script | `scripts/bench/bench_group.js` |
| Users | 20 |
| Group size | 20 |
| Senders | 1 |
| Messages per sender | 20 |
| Hot-group warm-up messages | 1 |
| Hot-group thresholds | members=20, messages=1 |
| Phone namespace | `181` |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 20 |
| Accepted | 20 |
| Rejected | 0 |
| Persisted | 20 |
| Received | 380 |
| Expected receipts | 380 |
| Accepted throughput | 1.17 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 237.35 ms |
| P50 | 219.50 ms |
| P95 | 275.10 ms |
| P99 | 505.00 ms |
| Maximum | 505.00 ms |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 0 | 0 | N/A |
| Group | 20 | 0 | 0.00 |

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 1 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
