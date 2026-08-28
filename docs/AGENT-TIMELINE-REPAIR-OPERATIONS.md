# Agent Task Timeline Repair 运维手册

本文用于在隔离或共享环境中启用 `agent-timeline-repair`，不改变默认服务拓扑。worker 只重放低敏 Timeline 事件，不读取消息正文，也不会删除 repair ledger。

worker 支持常驻轮询和显式 `-once` 两种模式。常驻模式适合 Compose service，`-once` 适合 CronJob、发布验证和受控故障演练；两者共享相同的 claim、重放、完成和 retry 语义。

## 前置检查

1. 确认当前镜像包含 `/app/dipole-agent-task-timeline-repair`，并记录发布 revision。
2. 确认 migration 已完成到 v49，`mysql-permissions` 使用已替换的 repair 账号密码。
3. 为共享环境注入 `DIPOLE_AGENT_TIMELINE_REPAIR_MYSQL_PASSWORD`，不要把真实密码写入仓库或命令历史。
4. 确认 `DIPOLE_INTERNAL_RPC_SHARED_SECRET` 已注入；Compose 配置检查应通过。
5. 启用前保存 repair ledger 数量、Prometheus 快照和当前告警状态。

## 隔离启用

```bash
docker compose --profile agent-timeline-repair up -d agent-timeline-repair
docker compose --profile agent-timeline-repair ps agent-timeline-repair
docker compose --profile agent-timeline-repair logs --tail=100 agent-timeline-repair
```

有界执行可直接运行一次批次：

```bash
/app/dipole-agent-task-timeline-repair -once -batch-size 100
```

退出码为 `0` 且摘要中的 `repaired`/`retried` 与窗口记录一致时，才可归档该批次；失败时保留 repair ledger 和错误日志，不手工改状态。

启用 observability profile 后，repair 指标由服务版 Prometheus 以可选目标抓取：

```bash
docker compose --profile agent-timeline-repair --profile observability up -d prometheus
curl -fsS http://127.0.0.1:9090/api/v1/query \
  --data-urlencode 'query=dipole_agent_task_timeline_repair_total'
```

验收至少记录：`repaired`、`projection_error`、`complete_error`、`claim_error`、`invalid`、`empty` 六类 outcome，以及 worker readiness、镜像 revision、观察窗口和 operator。短窗口失败告警属于 warning，连续 projection retry 告警属于 critical。

## 暂停与回切

发现错误或告警持续时，先暂停 worker，保留 ledger 供后续重放：

```bash
docker compose --profile agent-timeline-repair stop agent-timeline-repair
```

确认 `agent_task_timeline_repairs` 的 pending/processing 统计和最后错误后，再按原 revision 启动。回切不执行 down migration、不手工修改 `repair_status`，也不删除 Timeline 事件；重复重放依靠 `event_uuid` 幂等。

## 证据归档

每次灰度归档以下低敏信息：deployment revision、Compose profile、worker 参数、开始/结束时间、Prometheus 原始快照哈希、outcome 聚合、告警结果、暂停/恢复时间和回滚结论。缺少完整窗口、原始快照或回滚记录时，保持 worker 显式启用，不提升为默认生产开关。
