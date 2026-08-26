# Search 归档恢复手册

本文档描述 Search mutation 快照的创建、对象存储发布、指定版本恢复和 Elasticsearch 重建流程。归档 bucket 必须启用 versioning、object lock，并配置不低于 `storage.search_archive_retention_days` 的 Governance 保留策略。

## 1. 创建固定快照

在 Message mutation 生产者维护窗口内执行：

```bash
go run ./cmd/search-archive \
  --action create \
  --manifest /var/lib/dipole/search/search-v1.json \
  --snapshot-id search-v1-20260827 \
  --batch-size 500
```

命令固定当前 Message Outbox 高水位，将每个 Message 的最终 mutation 流式写入 NDJSON，并生成条目数和 SHA-256 manifest。已有输出不会被覆盖。

## 2. 发布到对象存储

```bash
go run ./cmd/search-archive \
  --action publish \
  --manifest /var/lib/dipole/search/search-v1.json \
  --receipt /var/lib/dipole/search/search-v1-receipt.json \
  --object-prefix search
```

发布顺序固定为 data 后 manifest。receipt 记录两个对象的 key、version ID、ETag、保留截止时间、snapshot ID、高水位和归档 hash，应作为发布证据单独保存。缺少 version ID、bucket versioning/object lock 或保留时间低于配置门槛时命令失败。

## 3. 按版本恢复

```bash
go run ./cmd/search-archive \
  --action restore \
  --receipt /var/lib/dipole/search/search-v1-receipt.json \
  --destination /var/lib/dipole/search/restored
```

恢复始终读取 receipt 指定的对象版本，随后重新校验 manifest、条目数、高水位和 SHA-256。不得用对象的 latest 版本替代 receipt。

## 4. 重建和切换

```bash
go run ./cmd/search-backfill \
  --job search-v1-20260827 \
  --target-index dipole-messages-v1-build-20260827 \
  --source archive \
  --archive-manifest /var/lib/dipole/search/restored/search-v1.json

go run ./cmd/search-reconcile \
  --job search-v1-20260827 \
  --target-index dipole-messages-v1-build-20260827 \
  --source archive \
  --archive-manifest /var/lib/dipole/search/restored/search-v1.json
```

Reconcile 必须返回一致报告后才能执行 `search-alias`。Backfill Job 会绑定 source kind、snapshot ID 与 hash，后续换源或使用不同归档会失败。

## 5. 回滚与清理边界

Alias 回滚继续使用原 Job 和原 receipt 恢复出的 manifest。归档发布与恢复成功只证明恢复源可用，当前版本尚未提供生产 Outbox 自动清理授权；不得手工批量删除业务库事件。清理能力上线前仍需保留 Outbox，并保存空索引重建、Reconcile 和 Alias 回滚演练报告。
