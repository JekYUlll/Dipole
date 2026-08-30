# Dipole Performance Baseline

Run ID: `c1-remote-group20`

Scenario: `group_blast`

Captured at: `20260830T024224Z`

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
| Senders | 1 |
| Messages per sender | 20 |
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
| Attempted | 10 |
| Accepted | 10 |
| Rejected | 0 |
| Persisted | 10 |
| Received | 190 |
| Expected receipts | 190 |
| Accepted throughput | 0.67 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 84.53 ms |
| P50 | 83.00 ms |
| P95 | 89.55 ms |
| P99 | 107.00 ms |
| Maximum | 107.00 ms |

## Process Resources

Samples: 7

Duration: 21.29 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 31.85% | 68.17 MiB | 9 | 191677 | 52 |
| Dipole Node2 | 19.35% | 65.68 MiB | 8 | 212845 | 27 |
| Dipole Node3 | 19.26% | 65.19 MiB | 8 | 212230 | 38 |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 0 | 0 | N/A |
| Group | 10 | 200 | 20.00 |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | yes |
| Conversation rows touched | 20 |
| Conversation messages observed | 10 |
| Conversation write operations | 10 |
| Conversation writes / observed message | 1.00 |
| Direct-message projection writes | 0 |
| Group-message projection writes | 10 |
| Group-init projection writes | 20 |
| Counter source | `dipole_conversation_projection_writes_total` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
| Direct message | 0 | 0 | N/A | N/A |
| Group message | 10 | 0 | 3.00 ms | 5.00 ms |
| Group init | 20 | 0 | 0.63 ms | 2.50 ms |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 0 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
