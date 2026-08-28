# Dipole Performance Baseline

Run ID: `c1-a6f-concurrent-20`

Scenario: `concurrent`

Captured at: `20260827T192904Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `a6f367fd67d79ace95c730388a5bd95ac70bcb1d` |
| CPU | AMD Ryzen 7 8845H w/ Radeon 780M Graphics |
| Topology | `docker-compose.dist.yml` |
| API base URL | `http://127.0.0.1:18081` |
| Node 1 WebSocket | `ws://127.0.0.1:18081` |
| Node 2 WebSocket | `ws://127.0.0.1:18082` |
| Benchmark script | `scripts/bench/bench.js` |
| Users | 20 |
| Group size | 50 |
| Senders | 20 |
| Messages per sender | 2 |
| Receiver connection window | 15000 ms |
| Sender connection window | 15000 ms |
| Hot-group warm-up messages | 0 |
| Hot-group thresholds | members=None, messages=None |
| Phone namespace | `139` |

### Runtime Provenance

Expected revision: `a6f367fd67d79ace95c730388a5bd95ac70bcb1d`

Source aligned: yes

| Service | Image ID | Revision | Source tree |
| --- | --- | --- | --- |
| Dipole Node1 | `138f8868b728` | `a6f367fd67d7` | clean |
| Dipole Node2 | `138f8868b728` | `a6f367fd67d7` | clean |
| Dipole Node3 | `138f8868b728` | `a6f367fd67d7` | clean |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 40 |
| Accepted | 40 |
| Rejected | 0 |
| Persisted | 40 |
| Received | 40 |
| Expected receipts | 40 |
| Accepted throughput | 2.51 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 707.98 ms |
| P50 | 739.00 ms |
| P95 | 1072.00 ms |
| P99 | 1118.45 ms |
| Maximum | 1136.00 ms |

## Process Resources

Samples: 7

Duration: 18.93 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 27.36% | 69.16 MiB | 8 | 175430 | 872 |
| Dipole Node2 | 17.96% | 65.48 MiB | 8 | 185987 | 691 |
| Dipole Node3 | 17.80% | 64.06 MiB | 8 | 189055 | 688 |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 40 | 80 | 2.00 |
| Group | 0 | 0 | N/A |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | yes |
| Conversation rows touched | 40 |
| Conversation messages observed | 40 |
| Conversation write operations | 160 |
| Conversation writes / observed message | 4.00 |
| Direct-message projection writes | 160 |
| Group-message projection writes | 0 |
| Group-init projection writes | 0 |
| Counter source | `dipole_conversation_projection_writes_total` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
| Direct message | 160 | 0 | 37.04 ms | 100.00 ms |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 26 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
