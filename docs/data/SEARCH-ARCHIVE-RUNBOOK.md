# Search 归档恢复手册

本文档描述 Search mutation 快照的创建、对象存储发布、指定版本恢复和 Elasticsearch 重建流程。归档 bucket 必须启用 versioning、object lock，并配置不低于 `storage.search_archive_retention_days` 的 Governance 保留策略。

## 1. 创建固定快照

在 Message mutation 生产者维护窗口内执行：

```bash
go run ./cmd/tools/search-archive \
  --action create \
  --manifest /var/lib/dipole/search/search-v1.json \
  --snapshot-id search-v1-20260827 \
  --batch-size 500
```

命令固定当前 Message Outbox 高水位，将每个 Message 的最终 mutation 流式写入 NDJSON，并生成条目数和 SHA-256 manifest。已有输出不会被覆盖。

## 2. 发布到对象存储

```bash
go run ./cmd/tools/search-archive \
  --action publish \
  --manifest /var/lib/dipole/search/search-v1.json \
  --receipt /var/lib/dipole/search/search-v1-receipt.json \
  --object-prefix search
```

发布顺序固定为 data 后 manifest。receipt 记录两个对象的 key、version ID、ETag、保留截止时间、snapshot ID、高水位和归档 hash，应作为发布证据单独保存。缺少 version ID、bucket versioning/object lock 或保留时间低于配置门槛时命令失败。

## 3. 按版本恢复

```bash
go run ./cmd/tools/search-archive \
  --action restore \
  --receipt /var/lib/dipole/search/search-v1-receipt.json \
  --destination /var/lib/dipole/search/restored
```

恢复始终读取 receipt 指定的对象版本，随后重新校验 manifest、条目数、高水位和 SHA-256。不得用对象的 latest 版本替代 receipt。

## 4. 重建和切换

```bash
go run ./cmd/tools/search-backfill \
  --job search-v1-20260827 \
  --target-index dipole-messages-v1-build-20260827 \
  --source archive \
  --archive-manifest /var/lib/dipole/search/restored/search-v1.json

go run ./cmd/tools/search-reconcile \
  --job search-v1-20260827 \
  --target-index dipole-messages-v1-build-20260827 \
  --source archive \
  --archive-manifest /var/lib/dipole/search/restored/search-v1.json
```

Reconcile 必须返回一致报告后才能执行 `search-alias`。Backfill Job 会绑定 source kind、snapshot ID 与 hash，后续换源或使用不同归档会失败。

## 5. 受控清理

先使用与目标 Backfill Job 对应的 receipt 和一致 Reconcile 报告执行 dry-run：

```bash
go run ./cmd/tools/search-outbox-cleanup \
  --receipt /var/lib/dipole/search/search-v1-receipt.json \
  --reconcile-report /var/lib/dipole/search/search-v1-reconcile.json \
  --target-index dipole-messages-v1-build-20260827 \
  --batch-size 500
```

确认 Message mutation 生产者已进入维护窗口，并核对 `eligible_count` 后执行清理。执行结果应重定向到只追加的审计存储：

```bash
go run ./cmd/tools/search-outbox-cleanup \
  --receipt /var/lib/dipole/search/search-v1-receipt.json \
  --reconcile-report /var/lib/dipole/search/search-v1-reconcile.json \
  --target-index dipole-messages-v1-build-20260827 \
  --batch-size 500 \
  --execute \
  --confirm-maintenance-window \
  --operator oncall@example.com \
  > /var/log/dipole/search-cleanup-20260827.json
```

执行模式强制要求责任人和维护窗口确认，只删除 receipt 高水位以内、已发布且属于八类 Search mutation 的 Message Outbox。范围内存在未发布 mutation 时整次拒绝；批次中断后可使用同一组证据重新执行，命令只处理剩余 eligible 行。生产部署使用 `configs/mysql/search-maintenance-grants.dist.sql` 创建独立账号，并通过 `search.mysql.*` 注入，禁止复用 Core 或 root 凭据。

## 6. 清理后恢复验收与回滚

清理后必须创建新的空物理索引，仅使用 receipt 恢复出的 archive 运行 Backfill 和 Reconcile；保存 100% hash 匹配报告。随后完成 Alias 正向切换和回滚演练。任何步骤失败时停止后续 Outbox 清理，保留原 Alias owner，并从同一 object version receipt 恢复。

不得手工批量删除 Outbox。审计记录至少保存 operator、snapshot ID、manifest/data version ID、Reconcile 时间、高水位、eligible 数和实际删除数。
