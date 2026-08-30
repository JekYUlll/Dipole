# Multipart 清理运行手册

## 范围

`dipole-multipart-cleanup` 同时检查 MinIO 未完成 Multipart 和可选的 Redis Multipart 会话异常。默认输出报告，不执行删除。

MinIO 列举过程中的任意错误都会将报告标记为 `complete=false` 并使命令失败，即使此前已经发现了部分候选。执行清理前必须确认报告完整；不完整报告只能保留为诊断证据，不能作为删除依据。

## 预览

先使用生产配置执行 MinIO dry-run：

```bash
go run ./cmd/tools/multipart-cleanup --older-than=24h
```

同时检查 Redis meta/parts：

```bash
go run ./cmd/tools/multipart-cleanup --older-than=24h --redis-orphans --redis-max-keys=10000
```

重点复核：`minio.candidates`、`redis.candidates`、`redis.complete` 和 `redis.errors`。`metadata_missing_ttl` 只进入人工复核，工具不会自动删除该 meta。

`redis.complete=false` 表示本次扫描达到上限或发生依赖错误，必须调整 `--redis-max-keys` 或修复依赖后重新预览；禁止在不完整报告上执行清理。

## Prometheus 文本指标

可以在 reconciliation 任务中选择性写出 Prometheus textfile collector 指标：

```bash
go run ./cmd/tools/multipart-cleanup \
  --reconcile \
  --metrics-output=/var/lib/node_exporter/textfile_collector/dipole_multipart_reconciliation.prom
```

该文件使用同目录临时文件写入并原子替换，避免采集到半份报告。指标只包含固定名称和低基数数值，不写入 `session_id`、对象键或其他上传标识。`--metrics-output` 只能与 `--reconcile` 一起使用，默认不会创建指标文件；JSON 报告和 `--reconcile-fail-on-drift` 的退出语义保持不变。

指标由定时 reconciliation 任务刷新。部署时应同时监控 textfile 文件的更新时间或
`dipole_multipart_reconciliation_last_run_timestamp_seconds`，避免任务停止后继续使用旧数据。

配套规则位于 `deploy/observability/multipart-alerts.yml`，包括：

- `DipoleMultipartReconciliationDrift`：发现 MinIO/Redis 跨存储漂移。
- `DipoleMultipartReconciliationIncomplete`：扫描未完整结束。
- `DipoleMultipartReconciliationStale`：指标超过 15 分钟未刷新。

这些规则只负责产生 Prometheus 告警；修复仍需先保留 JSON 报告，再按运维确认执行清理或回滚路径。

## 执行

确认维护窗口、对象前缀和报告后，才允许执行：

```bash
go run ./cmd/tools/multipart-cleanup \
  --older-than=24h \
  --redis-orphans \
  --execute \
  --confirm
```

执行范围包括 MinIO 过期 upload Abort 和 Redis 中无 meta 的 parts 删除。工具不会删除仍有 meta 的会话；任一分项失败会在 JSON 中保留错误并返回非零状态。

清理操作按 Redis `DEL` 语义设计，可安全重复执行；重复运行前仍应保留每次完整 JSON 报告。

## 回滚与复核

Redis parts 删除后无法从 Redis 恢复，执行前必须保存完整 dry-run JSON。MinIO Abort 后无法恢复原 upload，若报告不完整、依赖异常或出现未预期 key，应立即停止执行，继续使用 Core 中转上传，并通过对象存储审计和 Multipart 指标确认未产生新孤儿。
