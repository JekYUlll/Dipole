# Dipole Performance Baseline

Run ID: `g0-direct-20260827-final`

Scenario: `direct_msg`

Captured at: `20260827T013828Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `ec979d4e237ccf7d0158bf8ce01c96e896118a93` |
| CPU | AMD Ryzen 7 8845H w/ Radeon 780M Graphics |
| Topology | `docker-compose.dist.yml` |
| Benchmark script | `scripts/bench/bench.js` |
| Users | 20 |
| Group size | 2 |
| Senders | 10 |
| Messages per sender | 5 |
| Hot-group warm-up messages | 0 |
| Hot-group thresholds | members=None, messages=None |
| Phone namespace | `176` |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 50 |
| Accepted | 50 |
| Rejected | 0 |
| Persisted | 50 |
| Received | 50 |
| Expected receipts | 50 |
| Accepted throughput | 3.19 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 242.64 ms |
| P50 | 252.00 ms |
| P95 | 352.65 ms |
| P99 | 374.85 ms |
| Maximum | 392.00 ms |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 50 | 100 | 2.00 |
| Group | 0 | 0 | N/A |

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 8 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
