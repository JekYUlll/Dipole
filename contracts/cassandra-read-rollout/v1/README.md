# Cassandra Read Rollout Evidence v1

该合同用于归档 Message Service 的 Cassandra 主读比例、fallback、抽样核验与延迟聚合指标。Evaluator 只读取已采集的低敏 JSON，不访问数据库、Cassandra 或 Prometheus。

```bash
go run ./cmd/tools/cassandra-read-rollout-evidence -evidence=/path/evidence.json -policy=/path/policy.json
```

退出码 `0` 表示达到策略，`2` 表示有效证据未达到策略，`1` 表示输入无效。报告的 `eligible` 只代表窗口内证据满足门槛，不能替代真实共享环境责任人批准、快照、回放和回切演练。
