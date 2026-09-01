# Cassandra Read Rollout Evidence v1

该合同用于归档 Message Service 的 Cassandra 主读比例、fallback、抽样核验与延迟聚合指标。Evaluator 只读取已采集的低敏 JSON，不访问数据库、Cassandra 或 Prometheus。

先通过窗口采集脚本保存不可覆盖的两个 Prometheus 快照，再转换为 evidence JSON：

```bash
scripts/capture-cassandra-read-window.sh start
# 保持相同部署 revision 和 read percentage，完成受控观察窗口。
scripts/capture-cassandra-read-window.sh end

metadata=/secure/window/metadata.json
go run ./cmd/tools/cassandra-read-rollout-snapshot \
  -metrics-start=/secure/window/metrics-start.prom \
  -metrics-end=/secure/window/metrics-end.prom \
  -revision="$(jq -r .deploymentRevision "$metadata")" \
  -percentage="$(jq -r .configuredReadPercentage "$metadata")" \
  -window-start="$(jq -r .windowStart "$metadata")" \
  -window-end="$(cat /secure/window/window-end.txt)" \
  > /secure/window/evidence.json
```

转换器要求三个最终路由的累计计数和相应的延迟 histogram 保持单调。`mysql_fallback` 会同时计入 MySQL 最终路由与 fallback；抽样核验只接受 Router 当前产生的 `match`、`mismatch`、`mysql_error`。任一计数回退、标签漂移、bucket 变化或缺少覆盖已路由请求的 latency 会使转换失败。

```bash
go run ./cmd/tools/cassandra-read-rollout-evidence -evidence=/path/evidence.json -policy=/path/policy.json
```

退出码 `0` 表示达到策略，`2` 表示有效证据未达到策略，`1` 表示输入无效。报告的 `eligible` 只代表窗口内证据满足门槛，不能替代真实共享环境责任人批准、快照、回放和回切演练。
