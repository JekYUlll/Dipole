# Multipart 清理运行手册

## 范围

`dipole-multipart-cleanup` 同时检查 MinIO 未完成 Multipart 和可选的 Redis Multipart 会话异常。默认输出报告，不执行删除。

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
