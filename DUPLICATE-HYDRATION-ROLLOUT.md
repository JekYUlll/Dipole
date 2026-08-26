# Cassandra 重复消息 Hydration 灰度手册

本文档用于评估重复发送响应能否从 MySQL 正文安全迁移到 Metadata locator + Cassandra Timeline。规则测试只验证门禁表达式，真实晋级必须保存完整观察窗口证据。

## 部署前提

1. migration v14 已完成，`message_metadata.legacy_message_id` 已回填。
2. Cassandra Projector lag 为零，Backfill/Reconcile 对候选高水位一致。
3. Prometheus 抓取全部 Message Service `/metrics`，加载 `duplicate-hydration-alerts.yml`。
4. 先在受控 Message 节点启用 `message.cassandra_duplicate_hydration=true`，其余节点保留关闭配置用于即时回切。
5. 记录候选提交、节点、开始时间、结束时间、责任人和 Cassandra/MySQL 版本。

## 晋级门禁

完整 24 小时窗口内必须同时满足：

- `dipole:duplicate_hydration:window_complete == 1`；
- `dipole:duplicate_hydration:hits_24h >= 100`；
- `dipole:duplicate_hydration:fallbacks_24h == 0`；
- `dipole:duplicate_hydration:no_seq_24h == 0`；
- `dipole:duplicate_hydration:promotion_ready == 1`；
- Cassandra Projector、Backfill/Reconcile 和主读验证没有差异或依赖告警。

任意 fallback 表示请求仍需要 MySQL 正文，任意 `skipped_no_seq` 表示历史 Metadata 无法定位 Timeline。两者在正文退役前都必须为零。

## 停止与回切

以下任一条件出现时停止扩大范围，并将候选节点开关恢复为 `false`：

- `DipoleDuplicateHydrationFallbackRateHigh` firing；
- `DipoleDuplicateHydrationMissingSequence` firing；
- 24 小时后 `promotion_ready != 1`；
- Cassandra mismatch、Projector lag、retry 或 dead-letter 告警；
- 观察期间修改了 Metadata、Timeline payload 或幂等响应契约。

回切只改变 hydration 开关，重复响应立即恢复 MySQL 正文读取，不需要迁移数据。修复后发布新候选并重新执行完整窗口。

## 验证与归档

```bash
scripts/check-duplicate-hydration-alerts.sh
```

生产查询：

```promql
dipole:duplicate_hydration:promotion_ready
sum by (outcome) (increase(dipole_message_duplicate_hydration_total[24h]))
ALERTS{alertname=~"DipoleDuplicateHydration(FallbackRateHigh|MissingSequence)",alertstate="firing"}
```

归档 recording rules、两条告警状态、原始 outcome 增量、候选配置和责任人。完成本门禁仍不等于 MySQL 正文可以退役，Sync/旧 Offline 与整体 AD-019 条件必须独立完成。
