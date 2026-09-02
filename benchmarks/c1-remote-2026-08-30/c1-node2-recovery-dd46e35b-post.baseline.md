# Dipole Performance Baseline

Run ID: `c1-node2-recovery-dd46e35b-post`

Scenario: `concurrent`

Captured at: `20260830T025704Z`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `dd46e35bdece05cd23cce2ab6a04ee545c561105` |
| CPU | Intel(R) Xeon(R) Platinum 8481C |
| Topology | `deploy/compose/docker-compose.dist.yml` |
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

Expected revision: `dd46e35bdece05cd23cce2ab6a04ee545c561105`

Source aligned: yes

| Service | Image ID | Revision | Source tree |
| --- | --- | --- | --- |
| Dipole Node1 | `b55a64d7df44` | `dd46e35bdece` | clean |
| Dipole Node2 | `b55a64d7df44` | `dd46e35bdece` | clean |
| Dipole Node3 | `b55a64d7df44` | `dd46e35bdece` | clean |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | 40 |
| Accepted | 40 |
| Rejected | 0 |
| Persisted | 40 |
| Received | 40 |
| Expected receipts | 40 |
| Accepted throughput | 2.64 msg/s |
| Acceptance rate | 100.00% |
| Persistence rate | 100.00% |
| Delivery rate | 100.00% |
| HTTP failure rate | 0.00% |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | 71.10 ms |
| P50 | 102.50 ms |
| P95 | 113.05 ms |
| P99 | 115.83 ms |
| Maximum | 117.00 ms |

## Process Resources

Samples: 6

Duration: 17.44 s

Counter source: `/proc/<pid>/stat,status,task/*/status`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dipole Node1 | 35.09% | 70.90 MiB | 9 | 148291 | 46 |
| Dipole Node2 | 19.21% | 69.61 MiB | 9 | 169095 | 43 |
| Dipole Node3 | 19.32% | 68.32 MiB | 9 | 168029 | 39 |

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
| Direct message | 160 | 0 | 0.87 ms | 2.50 ms |
| Group message | 0 | 0 | N/A | N/A |
| Group init | 0 | 0 | N/A | N/A |

Duration source: `dipole_conversation_projection_write_duration_seconds`

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | 0 |
| Settled sampled lag | 0 |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
