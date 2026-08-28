# AD-005 1000 成员 Conversation SQL 对照

本目录归档候选提交 `4ac1540e4196c951f432be0ad4f79e27a450bf42` 在真实 MySQL 8.4.8 上的 1000 成员 Conversation projection 对照。benchmark 使用独立 contract database，避免污染运行中的 Dipole 数据库。

## Workload

```text
members: 1000
operations per case: 1
serial: 1 message x 1000 member-level upserts
batch: 1 message x 1 INSERT ... SELECT upsert
parallel-serial: RunParallel, each operation x 1000 member-level upserts
parallel-batch: RunParallel, each operation x 1 INSERT ... SELECT upsert
command: CGO_ENABLED=0 go test -count=1 -run '^$' -bench '^BenchmarkConversationGroupProjection1000$' -benchtime=1x -v ./internal/data/mysql/repository
```

## Result

| Case | Time/op | InnoDB row lock waits | InnoDB lock time | Projection rows |
| --- | ---: | ---: | ---: | ---: |
| serial | 9919.866 ms | 0 | 0 ms | 1000 |
| batch | 265.776 ms | 0 | 0 ms | 1000 |
| parallel-serial | 11734.013 ms | 0 | 0 ms | 1000 |
| parallel-batch | 33.162 ms | 0 | 0 ms | 1000 |

相同数据库层 workload 下，batch 相比 serial 约降低 37.3 倍；并发对照约降低 353.8 倍。该结果证明 SQL 投影层的写放大优化有效，不能直接推导端到端 WebSocket 投递 P95 的同等比例收益。

## Verification

- `PASS`，四个 benchmark 子项均完成。
- 每个子项校验 `conversations` 行数为 1000，且 `last_message_seq` 为正数。
- `Innodb_row_lock_waits` 和 `Innodb_row_lock_time` 记录为 benchmark 前后全局状态增量，四个子项均为零。
- 环境：Go 1.27.0、Linux amd64、16 CPU、MySQL 8.4.8、`CGO_ENABLED=0`。

原始终端输出见 [`raw-output.txt`](raw-output.txt)。单次 benchmark 只用于固定 workload 的结构对照；正式容量规划仍需要多轮采样、共享拓扑和端到端 k6 证据。
