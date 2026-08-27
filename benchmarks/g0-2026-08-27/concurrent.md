# Dipole Performance Baseline

Run ID: `g0-concurrent-20260827-final`

Scenario: `concurrent`

Captured at: `20260827T013851Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `ec979d4e237ccf7d0158bf8ce01c96e896118a93` |
| CPU | AMD Ryzen 7 8845H w/ Radeon 780M Graphics |
| Topology | `docker-compose.dist.yml` |
| Benchmark script | `scripts/bench/bench.js` |
| Users | 20 |
| Group size | 2 |
| Senders | 20 |
| Messages per sender | 8 |
| Hot-group warm-up messages | 0 |
| Hot-group thresholds | members=None, messages=None |
| Phone namespace | `178` |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 160 |
| Accepted | 160 |
| Rejected | 0 |
| Persisted | 160 |
| Received | 160 |
| Expected receipts | 160 |
| Accepted throughput | 10.39 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 1270.73 ms |
| P50 | 1356.00 ms |
| P95 | 1935.50 ms |
| P99 | 2071.04 ms |
| Maximum | 2129.00 ms |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 160 | 320 | 2.00 |
| Group | 0 | 0 | N/A |

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 28 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
