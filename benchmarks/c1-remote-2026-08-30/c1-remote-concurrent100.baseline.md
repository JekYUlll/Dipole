# Dipole Performance Baseline

Run ID: `c1-remote-concurrent100`

Scenario: `concurrent`

Captured at: `20260830T024622Z`

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
| Users | 100 |
| Group size | 20 |
| Senders | 100 |
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
| Attempted | 400 |
| Accepted | 400 |
| Rejected | 0 |
| Persisted | 400 |
| Received | 400 |
| Expected receipts | 400 |
| Accepted throughput | 14.51 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 127.31 ms |
| P50 | 149.00 ms |
| P95 | 178.05 ms |
| P99 | 243.01 ms |
| Maximum | 247.00 ms |

## Process Resources

Samples: 10

Duration: 33.10 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 61.03% | 75.36 MiB | 9 | 230482 | 109 |
| Dipole Node2 | 20.12% | 72.89 MiB | 9 | 334530 | 50 |
| Dipole Node3 | 19.46% | 67.86 MiB | 9 | 326412 | 45 |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 400 | 800 | 2.00 |
| Group | 0 | 0 | N/A |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | yes |
| Conversation rows touched | 200 |
| Conversation messages observed | 400 |
| Conversation write operations | 1200 |
| Conversation writes / observed message | 3.00 |
| Direct-message projection writes | 1200 |
| Group-message projection writes | 0 |
| Group-init projection writes | 0 |
| Counter source | `dipole_conversation_projection_writes_total` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
| Direct message | 1200 | 0 | 0.74 ms | 2.50 ms |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 43 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
