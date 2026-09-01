# Dipole Performance Baseline

Run ID: `c1-remote-20`

Scenario: `direct_msg`

Captured at: `20260830T023919Z`

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
| Group size | 50 |
| Senders | 10 |
| Messages per sender | 5 |
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
| Attempted | 50 |
| Accepted | 50 |
| Rejected | 0 |
| Persisted | 50 |
| Received | 50 |
| Expected receipts | 50 |
| Accepted throughput | 3.11 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 70.04 ms |
| P50 | 49.00 ms |
| P95 | 162.10 ms |
| P99 | 165.00 ms |
| Maximum | 165.00 ms |

## Process Resources

Samples: 7

Duration: 21.29 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 32.74% | 67.18 MiB | 8 | 193298 | 90 |
| Dipole Node2 | 20.01% | 66.51 MiB | 8 | 213479 | 43 |
| Dipole Node3 | 19.63% | 64.16 MiB | 9 | 214032 | 18 |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | 50 | 100 | 2.00 |
| Group | 0 | 0 | N/A |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | yes |
| Conversation rows touched | 20 |
| Conversation messages observed | 50 |
| Conversation write operations | 140 |
| Conversation writes / observed message | 2.80 |
| Direct-message projection writes | 140 |
| Group-message projection writes | 0 |
| Group-init projection writes | 0 |
| Counter source | `dipole_conversation_projection_writes_total` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
| Direct message | 140 | 0 | 0.84 ms | 2.50 ms |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 0 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
