# Agent OpenTelemetry 运维说明

## 范围与默认值

`observability` profile 提供单节点 OpenTelemetry Collector `0.159.0` 与 Tempo `2.10.5`。Agent exporter 仍由 `DIPOLE_AGENT_OTEL_ENABLED=false` 默认关闭。Collector 只接收 OTLP/HTTP protobuf，经过 128 MiB memory limiter 与有界 batch/queue 后写入 Tempo。Tempo 使用独立本地卷和 24 小时 block retention；该 local backend 用于开发、Shadow 验收和故障演练，共享生产环境需要改用具备生命周期策略的对象存储。

所有宿主端口只绑定 `127.0.0.1`：Collector OTLP `4318`、健康检查 `13133`、内部指标 `8888`，Tempo 查询 `3200`。不要将这些端口直接暴露到公网。

## 启用与验收

```bash
export DIPOLE_INTERNAL_RPC_SHARED_SECRET='local-only-value'
scripts/check-agent-otel-observability.sh
scripts/smoke-agent-otel.sh

export DIPOLE_AGENT_OTEL_ENABLED=true
docker compose -f deploy/compose/docker-compose.microservices.yml --profile observability up -d tempo otel-collector prometheus
```

Smoke 使用固定 localhost 验收端口、随机 Compose project 和测试专用卷，因此应在长期 profile 启动前独立执行。它生成 `agent.otel.smoke` span，验证 Collector accepted/sent 指标并按 trace ID 从 Tempo 查询完整 trace，退出时删除测试栈。Compose 启动默认限制为 300 秒；可用 `DIPOLE_AGENT_OTEL_SMOKE_STARTUP_TIMEOUT_SECONDS` 在 30 至 1800 秒内调整，超时同样执行清理。正式环境应另外归档镜像 digest、Prometheus 原始查询、Tempo trace ID、执行时间和配置 commit。

## Trace 与审计联查

Agent span 只包含低敏 `task.id`、`run.id`、阶段、route、Capability、计数和状态。操作员先从告警或 Tempo 获取 trace，再使用 span 中的 Task/Run ID，通过受控只读接口或审计账号查询：

```sql
SELECT task_uuid, status, trigger_type, created_at, updated_at
FROM agent_tasks
WHERE task_uuid = ?;

SELECT run_uuid, status, runtime_id, started_at, completed_at
FROM agent_runs
WHERE task_uuid = ? AND run_uuid = ?;

SELECT step_no, capability_id, status, attempt_count, started_at, finished_at
FROM agent_shadow_steps
WHERE task_uuid = ?
ORDER BY step_no;
```

查询结果不得复制 Prompt、消息正文、Tool 参数/结果或 Artifact 正文到告警标签和工单标题。需要内容证据时沿既有授权 API 单独读取，并保留访问审计。

## 告警与回滚

- `DipoleAgentOTelCollectorDown`：Collector 指标端点持续一分钟不可达。
- `DipoleAgentOTelExportFailures`：五分钟内向 Tempo 发送失败。
- `DipoleAgentOTelRefusedSpans`：memory limiter 在五分钟内拒绝 span。

先关闭 `DIPOLE_AGENT_OTEL_ENABLED` 并滚动 Agent，即可停止新 trace 开销；Agent 业务链路继续运行。随后停止 `otel-collector` 和 `tempo`。测试或开发环境确认无需保留证据后才删除 Tempo 卷；共享环境必须按保留审批和对象存储生命周期执行。Collector/Tempo 故障不得改变 Agent readiness，也不得阻断消息和 Task 处理。
