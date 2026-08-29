# AD-005 1000 成员 Conversation SQL 复测

本目录记录 `master` 提交 `3adc755` 在本地 MySQL 8.4 实例上的固定 workload 复测结果，用于和 2026-08-29 的 SQLC 批量 upsert 基线对照。

## Workload

```text
members: 1000
operations per case: 1
serial: 1 message x 1000 member-level upserts
batch: 1 message x 1 INSERT ... SELECT upsert
parallel-serial: RunParallel, each operation x 1000 member-level upserts
parallel-batch: RunParallel, each operation x 1 INSERT ... SELECT upsert
command: CGO_ENABLED=0 DIPOLE_TEST_MYSQL_ADMIN_DSN='root:root123@tcp(127.0.0.1:3306)/?parseTime=true&loc=UTC' go test -count=1 -run '^$' -bench '^BenchmarkConversationGroupProjection1000$' -benchtime=1x -v ./internal/services/core/infrastructure/mysql
```

## Result

| Case | Time/op | InnoDB row lock waits | InnoDB lock time | Projection rows |
| --- | ---: | ---: | ---: | ---: |
| serial | 16765.257 ms | 0 | 0 ms | 1000 |
| batch | 362.631 ms | 0 | 0 ms | 1000 |
| parallel-serial | 20172.274 ms | 0 | 0 ms | 1000 |
| parallel-batch | 70.327 ms | 0 | 0 ms | 1000 |

本次固定实例中，batch 相比 serial 约降低 46.2 倍；并发对照约降低 286.9 倍。绝对耗时受共享本地实例状态影响，不能与不同运行窗口直接作容量结论。

## Verification

- `PASS`，四个 benchmark 子项均完成。
- 每个子项校验 `conversations` 行数为 1000，且 `last_message_seq` 为正数。
- `Innodb_row_lock_waits` 和 `Innodb_row_lock_time` 的 benchmark 前后增量均为零。
- 环境：Go 1.27.0、Linux amd64、16 CPU、MySQL 8.4、`CGO_ENABLED=0`。
- 本次仍为单轮 SQL 层复测；端到端 P95、多轮统计和共享拓扑容量验证继续由 AD-005 跟踪。

原始终端输出见 [`raw-output.txt`](raw-output.txt)。
