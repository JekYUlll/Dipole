# Dipole Performance Baseline

Run ID: `c1-ce4b600-node2-recovery-post`

Scenario: `concurrent`

Captured at: `20260827T194917Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `ce4b6005650b5c29e1f01d3955ae3d0491559bba` |
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

Expected revision: `ce4b6005650b5c29e1f01d3955ae3d0491559bba`

Source aligned: yes

| Service | Image ID | Revision | Source tree |
| --- | --- | --- | --- |
| Dipole Node1 | `651fd9f80f1b` | `ce4b6005650b` | clean |
| Dipole Node2 | `651fd9f80f1b` | `ce4b6005650b` | clean |
| Dipole Node3 | `651fd9f80f1b` | `ce4b6005650b` | clean |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 40 |
| Accepted | 40 |
| Rejected | 0 |
| Persisted | 40 |
| Received | 40 |
| Expected receipts | 40 |
| Accepted throughput | 2.41 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 908.67 ms |
| P50 | 993.00 ms |
| P95 | 1363.40 ms |
| P99 | 1413.79 ms |
| Maximum | 1429.00 ms |

## Process Resources

Samples: 7

Duration: 18.65 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 27.23% | 70.21 MiB | 8 | 173903 | 932 |
| Dipole Node2 | 17.85% | 65.18 MiB | 8 | 192091 | 811 |
| Dipole Node3 | 17.80% | 64.03 MiB | 8 | 191745 | 765 |

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
| Direct message | 160 | 0 | 46.94 ms | 100.00 ms |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 4 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
