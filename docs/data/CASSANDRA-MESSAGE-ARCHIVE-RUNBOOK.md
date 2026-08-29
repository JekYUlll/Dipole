# Cassandra 完整消息归档运行手册

本文档用于在 MySQL 历史消息清理或 Cassandra 灾难恢复前，创建、保留并验证可独立重建 Timeline 的完整消息快照。

## 前置条件

- 先执行最新 SQL migration；migration v15 会把 Cassandra Backfill Job 绑定到 source kind、snapshot ID 与 SHA-256。
- `storage.message_archive_bucket` 必须启用 versioning 与 object lock，默认 Governance 保留期不得低于 `storage.message_archive_retention_days`。
- 操作人员应记录变更单、Job 名、snapshot ID、receipt 路径、对象 Version ID、保留截止时间和回滚责任人。
- 在 Reconcile 一致、对象恢复演练完成和业务观察窗口结束前，禁止删除 MySQL 正文。

## 创建与发布

```bash
dipole-cassandra-archive \
  -action create \
  -manifest /backup/message-20260827.json \
  -snapshot-id message-20260827 \
  -batch-size 500

dipole-cassandra-archive \
  -action publish \
  -manifest /backup/message-20260827.json \
  -receipt /backup/message-20260827.receipt.json
```

receipt 固定 manifest 与 NDJSON 的 bucket、object key、Version ID 和 ETag。保留 receipt，并在隔离目录执行一次恢复：

```bash
dipole-cassandra-archive \
  -action restore \
  -receipt /backup/message-20260827.receipt.json \
  -destination /restore/message-20260827
```

## 重建与对账

同一个 Job 的 Backfill 与 Reconcile 必须使用同一恢复 manifest：

```bash
dipole-cassandra-backfill \
  -job message-timeline-20260827 \
  -source archive \
  -archive-manifest /restore/message-20260827/message-20260827.json

dipole-cassandra-reconcile \
  -job message-timeline-20260827 \
  -source archive \
  -archive-manifest /restore/message-20260827/message-20260827.json \
  -sample-modulus 1
```

Reconcile 退出码 `0` 表示一致，`2` 表示发现缺失、hash、抽样内容或 Seq 连续性差异。任何非零结果都应停止清理或切流。

## 回滚与约束

- 需要切回 MySQL 源时创建新的 Job 名；source identity 不允许在已有 Job 上覆盖。
- 归档文件和 receipt 采用创建即不可覆盖语义；重新导出使用新的 snapshot ID 和路径。
- Cassandra 重建失败时保留原表和 MySQL 正文，修复后以相同 Job/manifest 续跑；checkpoint 只在整批成功后前移。
- Object Lock 保留期、schema、字段集合或 Timeline payload hash 规则变化时，必须重新执行归档恢复 smoke 并更新本手册。

自动验收命令：

```bash
scripts/smoke-cassandra-message-archive.sh
```
