# Performance Baseline

本文档滚动记录可重复的性能基线。微基准用于比较协议与实现开销，端到端基准用于固定消息接受、持久化、投递、Kafka lag 与 Inbox 写放大行为。

## 2026-08-27：AD-005 Conversation Projection Timing

候选提交 `4343684011a02112eb3e9233e7c4279bf64a4ee9` 在 Service-to-Repository 窄边界记录 `projection × success|error` Histogram。operations/baseline v3 逐节点保存前后快照，先检测 Counter 回退，再聚合成功次数、累计耗时、平均耗时和 P95 桶上界；v1/v2 报告继续可读并明确标记 timing unavailable。

| Scenario | Members | Group-message calls | Average call | P95 bucket upper bound | Inbox writes/message | Delivery | End-to-end P95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Regular | 20 | 400 | 14.07 ms | 50 ms | 20 | 380/380 | 693.45 ms |
| Hot | 20 | 420 | 12.43 ms | 25 ms | 0 | 380/380 | 329.60 ms |
| Regular | 100 | 1000 | 23.07 ms | 50 ms | 100 | 990/990 | 8189.00 ms |
| Hot | 100 | 1100 | 21.97 ms | 50 ms | 0 | 990/990 | 1346.00 ms |

四组成功 Counter 与 Histogram 次数一致、错误为 0、Kafka settled lag 为 0。普通与热群的单次 Conversation 调用分布接近，同规模端到端差异仍很大；Inbox/投递路径是模式差异的重要来源。Conversation 的成员级串行调用累计耗时仍随群规模增长，AD-005 保持处理中，下一步在 1000 人固定 workload 或候选批处理/读扩散方案上评估收益和回滚语义。

100 人场景显式使用 22 秒 receiver window，20 人场景为 15 秒，均受 25 秒 k6 场景上限约束。两次 15 秒的 100 人普通群样本因 receipt 不完整被门禁拒绝，没有进入归档。规范化报告、operations、k6 summary、Kafka lag、派生 timing、24 份逐节点原始 Prometheus 快照和 SHA-256 位于 `benchmarks/ad005-projection-timing-2026-08-27/`；节点已恢复默认热群阈值 `200/50`。

## 2026-08-27：AD-005 Conversation Projection Amplification

候选提交 `2202f1f5dd022a27603656d32a205ed9939de9cf` 增加 `dipole_conversation_projection_writes_total`，三个 Core 节点分别按 `direct_message|group_message|group_init` 计数成功 SQL upsert。基准脚本在 workload 前后求和取差值，并以同一 run namespace 的全部消息作为分母。

| Scenario | Members | Measured messages | Conversation writes/message | Inbox writes/message | Delivery | P95 | Kafka settled lag |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Regular | 20 | 20 | 20 | 20 | 380/380 | 1924.15 ms | 0 |
| Hot | 20 | 20 + 1 warm-up | 20 | 0 | 380/380 | 668.80 ms | 0 |
| Regular | 100 | 10 | 100 | 100 | 990/990 | 10065.00 ms | 0 |
| Hot | 100 | 10 + 1 warm-up | 100 | 0 | 990/990 | 1853.00 ms | 0 |

结果确认普通群与热群的 Conversation State 写扩散都随成员数线性增长。热群将 Inbox 扩散降为零后，本次 100 人 P95 明显下降；两条链路仍包含投递策略差异，因此该对照不足以把剩余延迟单独归因于 Conversation SQL。AD-005 继续处理中，下一阶段需补充 projection 级数据库耗时，再决定读扩散或批处理。

规范化 JSON、参数、解释边界与 SHA-256 位于 `benchmarks/ad005-2026-08-27/`。节点在采集后已恢复默认热群阈值 `200/50`。

## 2026-08-27：G0 End-to-End Message Flow

环境：

```text
CPU: AMD Ryzen 7 8845H
OS/Arch: linux/amd64
Commit: ec979d4e237ccf7d0158bf8ce01c96e896118a93
Topology: docker-compose.dist.yml
Application: 3 Dipole nodes
Infrastructure: MySQL 8.4, Redis 7.4, Kafka 3.9.0
Clients: local k6, 20 WebSocket connections per scenario
```

结果：

| Scenario | Attempted / accepted / persisted | Delivery | Accepted msg/s | P95 | P99 | Peak / settled Kafka lag | Inbox amplification |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Direct, 10 senders x 5 | 50 / 50 / 50 | 100% | 3.19 | 352.65 ms | 374.85 ms | 8 / 0 | 2x |
| Concurrent ring, 20 x 8 | 160 / 160 / 160 | 100% | 10.39 | 1,935.50 ms | 2,071.04 ms | 28 / 0 | 2x |
| Regular group, 1 x 20 | 20 / 20 / 20 | 100% (380/380) | 1.17 | 286.10 ms | 289.00 ms | 1 / 0 | 20x |
| Hot group, 1 x 20 | 20 / 20 / 20 | 100% (380/380) | 1.17 | 275.10 ms | 505.00 ms | 1 / 0 | 0x |

Direct 与 concurrent 的 `2x` 对应发送者和接收者两侧 Inbox；普通 20 人群的 `20x` 对应每个成员一行。热群先用一条不计入测量的消息进入热态，随后 20 条测量消息通过聚合通知和历史补拉完成 380 次接收，未生成 Inbox 行。完整标准化 JSON 与 Markdown 位于 `benchmarks/g0-2026-08-27/`。

这些 workload 使用固定发送间隔验证链路正确性和可重复性，`msg/s` 反映本次负载节奏，不能解释为系统饱和吞吐。Concurrent P95 明显高于 direct，后续扩大连接数或提高发送速率时应继续记录 CPU、RSS 和投递尾延迟，再据此决定 AD-005 和 C++ 数据面优化。

复现入口：

```bash
SCENARIO=direct_msg SCENARIO_FILTER=direct_msg \
RUN_ID=<unique-run-id> PHONE_PREFIX=<unused-prefix> \
./scripts/bench/run_bench.sh

SCENARIO=group_regular BENCH_SCRIPT=scripts/bench/bench_group.js \
RUN_ID=<unique-run-id> PHONE_PREFIX=<unused-prefix> GROUP_SIZE=20 \
HOT_GROUP_MEMBER_COUNT_THRESHOLD=200 HOT_GROUP_MESSAGE_THRESHOLD=50 \
./scripts/bench/run_bench.sh
```

热群对照需要通过 Compose 将三个应用节点的阈值临时覆盖为 `20/1`，并为运行脚本设置相同的报告参数和 `HOT_GROUP_WARMUP_MESSAGES=1`；采集结束后必须用默认环境重建节点并确认阈值恢复为 `200/50`。

## 2026-08-26：Message History Transport

环境：

```text
CPU: AMD Ryzen 7 8845H
OS/Arch: linux/amd64
Transport: loopback TCP
Security: TLS 1.3 mTLS + service secret + certificate caller binding
Payload: one direct-history Message
```

命令：

```bash
LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
go test ./internal/bootstrap \
  -run '^$' \
  -bench '^BenchmarkMessageTransportDirectHistory$' \
  -benchmem -benchtime=2s -count=3
```

结果：

| Transport | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Local | 50.28-50.72 | 296 | 2 |
| gRPC mTLS | 69,339-69,618 | 14,932-14,934 | 243 |

M4 loopback adapter 门槛为平均 `<1 ms/op`，本次三轮 Remote 结果约 `0.0695 ms/op`，通过门槛。该结果不代表公网或跨节点端到端 P95/P99；后续 C++ Gateway/Delivery 对比必须复用独立的连接数、吞吐、P50/P95/P99、CPU 与 RSS 基线。
