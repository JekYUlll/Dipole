# Sync Cassandra Hydration Evidence v1

该合同归档 Sync Service 的 Cassandra hydration 命中、MySQL fallback、缺失/冲突/错误与 p95 延迟。Evaluator 只处理低敏聚合 JSON，不访问运行中的数据库或 Prometheus。

```bash
go run ./cmd/sync-cassandra-hydration-evidence -evidence=/path/evidence.json -policy=/path/policy.json
```

运行时快照可先转换为同一 evidence 输入：

```bash
go run ./cmd/sync-cassandra-hydration-snapshot \
  -metrics=/path/sync.metrics \
  -service=sync-service -revision=sync@REVISION -mode=primary \
  -window-start=2026-08-29T00:00:00Z -window-end=2026-08-29T01:00:00Z \
  > /path/evidence.json
```

转换器只信任显式传入的窗口元数据，不把 Prometheus 标签当作身份来源；取消请求归入 `error`，hit p95 使用满足 95% 样本的有限桶上界，无法计算时拒绝生成 evidence。

`shadow` 和 `primary` 均需经过相同门禁；退出码 `0/2/1` 分别表示达标、门禁失败和输入无效。`eligible` 只表示观测窗口满足策略，不能单独开启生产主读或停止 MySQL fallback。
