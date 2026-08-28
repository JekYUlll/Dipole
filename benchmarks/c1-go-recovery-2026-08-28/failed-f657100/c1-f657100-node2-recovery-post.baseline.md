# Dipole Performance Baseline

Run ID: `c1-f657100-node2-recovery-post`

Scenario: `concurrent`

Captured at: `20260827T194243Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `f6571004563b628cadf452abd6cf12582fcaa590` |
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
| Phone namespace | `136` |

### Runtime Provenance

Expected revision: `f6571004563b628cadf452abd6cf12582fcaa590`

Source aligned: yes

| Service | Image ID | Revision | Source tree |
| --- | --- | --- | --- |
| Dipole Node1 | `89cdc8e8086f` | `f6571004563b` | clean |
| Dipole Node2 | `89cdc8e8086f` | `f6571004563b` | clean |
| Dipole Node3 | `89cdc8e8086f` | `f6571004563b` | clean |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 40 |
| Accepted | 40 |
| Rejected | 0 |
| Persisted | 0 |
| Received | 0 |
| Expected receipts | 40 |
| Accepted throughput | 3.06 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 0.00% |
| Delivery rate | 0.00% |
| HTTP failure rate | 30.65% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | N/A |
| P50 | N/A |
| P95 | N/A |
| P99 | N/A |
| Maximum | N/A |

## Process Resources

Samples: 6

Duration: 15.92 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 14.01% | 65.53 MiB | 8 | 61144 | 939 |
| Dipole Node2 | 7.73% | 64.62 MiB | 9 | 58709 | 818 |
| Dipole Node3 | 7.85% | 62.10 MiB | 8 | 57906 | 797 |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 0 | 0 | N/A |
| Group | 0 | 0 | N/A |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | yes |
| Conversation rows touched | 0 |
| Conversation messages observed | 0 |
| Conversation write operations | 0 |
| Conversation writes / observed message | N/A |
| Direct-message projection writes | 0 |
| Group-message projection writes | 0 |
| Group-init projection writes | 0 |
| Counter source | `dipole_conversation_projection_writes_total` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
| Direct message | 0 | 0 | N/A | N/A |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 0 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
