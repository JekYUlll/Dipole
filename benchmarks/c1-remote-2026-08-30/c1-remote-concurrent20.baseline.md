# Dipole Performance Baseline

Run ID: `c1-remote-concurrent20`

Scenario: `concurrent`

Captured at: `20260830T024415Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `160d2cc620ac62de33b099a1690a2d84e6a8bb18` |
| CPU | Intel(R) Xeon(R) Platinum 8481C |
| Topology | `deploy/compose/docker-compose.dist.yml` |
| API base URL | `http://127.0.0.1:18081` |
| Node 1 WebSocket | `ws://127.0.0.1:18081` |
| Node 2 WebSocket | `ws://127.0.0.1:18082` |
| Benchmark script | `scripts/bench/bench.js` |
| Users | 20 |
| Group size | 20 |
| Senders | 20 |
| Messages per sender | 4 |
| Receiver connection window | 15000 ms |
| Sender connection window | 15000 ms |
| Hot-group warm-up messages | 0 |
| Hot-group thresholds | members=None, messages=None |
| Phone namespace | `138` |

### Runtime Provenance

Expected revision: `160d2cc620ac62de33b099a1690a2d84e6a8bb18`

Source aligned: yes

| Service | Image ID | Revision | Source tree |
| --- | --- | --- | --- |
| Dipole Node1 | `9154bba351e3` | `160d2cc620ac` | clean |
| Dipole Node2 | `9154bba351e3` | `160d2cc620ac` | clean |
| Dipole Node3 | `9154bba351e3` | `160d2cc620ac` | clean |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 80 |
| Accepted | 80 |
| Rejected | 0 |
| Persisted | 80 |
| Received | 80 |
| Expected receipts | 80 |
| Accepted throughput | 5.28 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 65.41 ms |
| P50 | 91.50 ms |
| P95 | 103.05 ms |
| P99 | 104.42 ms |
| Maximum | 106.00 ms |

## Process Resources

Samples: 6

Duration: 17.55 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 34.41% | 70.11 MiB | 8 | 152635 | 38 |
| Dipole Node2 | 18.57% | 65.86 MiB | 9 | 168420 | 14 |
| Dipole Node3 | 18.40% | 64.80 MiB | 8 | 168118 | 14 |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 80 | 160 | 2.00 |
| Group | 0 | 0 | N/A |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | yes |
| Conversation rows touched | 40 |
| Conversation messages observed | 80 |
| Conversation write operations | 240 |
| Conversation writes / observed message | 3.00 |
| Direct-message projection writes | 240 |
| Group-message projection writes | 0 |
| Group-init projection writes | 0 |
| Counter source | `dipole_conversation_projection_writes_total` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
| Direct message | 240 | 0 | 0.80 ms | 2.50 ms |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 0 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
