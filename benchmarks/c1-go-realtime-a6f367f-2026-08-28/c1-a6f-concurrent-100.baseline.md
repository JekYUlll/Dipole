# Dipole Performance Baseline

Run ID: `c1-a6f-concurrent-100`

Scenario: `concurrent`

Captured at: `20260827T193013Z`

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
| Users | 100 |
| Group size | 50 |
| Senders | 100 |
| Messages per sender | 2 |
| Receiver connection window | 15000 ms |
| Sender connection window | 15000 ms |
| Hot-group warm-up messages | 0 |
| Hot-group thresholds | members=None, messages=None |
| Phone namespace | `136` |

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
| Attempted | 200 |
| Accepted | 200 |
| Rejected | 0 |
| Persisted | 200 |
| Received | 200 |
| Expected receipts | 200 |
| Accepted throughput | 4.85 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 4959.01 ms |
| P50 | 5299.00 ms |
| P95 | 8084.00 ms |
| P99 | 8382.31 ms |
| Maximum | 8475.00 ms |

## Process Resources

Samples: 15

Duration: 46.52 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 36.99% | 72.62 MiB | 8 | 388726 | 3127 |
| Dipole Node2 | 18.29% | 71.62 MiB | 9 | 465081 | 1927 |
| Dipole Node3 | 18.08% | 67.51 MiB | 8 | 467543 | 1962 |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 200 | 400 | 2.00 |
| Group | 0 | 0 | N/A |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | yes |
| Conversation rows touched | 200 |
| Conversation messages observed | 200 |
| Conversation write operations | 800 |
| Conversation writes / observed message | 4.00 |
| Direct-message projection writes | 800 |
| Group-message projection writes | 0 |
| Group-init projection writes | 0 |
| Counter source | `dipole_conversation_projection_writes_total` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
| Direct message | 800 | 0 | 64.15 ms | 250.00 ms |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 158 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
