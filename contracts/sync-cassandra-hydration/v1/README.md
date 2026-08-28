# Sync Cassandra Hydration Evidence v1

该合同归档 Sync Service 的 Cassandra hydration 命中、MySQL fallback、缺失/冲突/错误与 p95 延迟。Evaluator 只处理低敏聚合 JSON，不访问运行中的数据库或 Prometheus。

```bash
go run ./cmd/sync-cassandra-hydration-evidence -evidence=/path/evidence.json -policy=/path/policy.json
```

`shadow` 和 `primary` 均需经过相同门禁；退出码 `0/2/1` 分别表示达标、门禁失败和输入无效。`eligible` 只表示观测窗口满足策略，不能单独开启生产主读或停止 MySQL fallback。
