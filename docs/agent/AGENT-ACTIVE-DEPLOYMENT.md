# Agent Active 部署运行手册

本文档约束 `read_active` 的 user-gray 部署。基础 Compose 固定为 Shadow；只有显式加载 active overlay 才会请求 active 依赖。

## 1. 边界

`docker compose config` 通过证明部署输入完整。它不提供 Kafka、Temporal、Capability RPC、模型 Provider、评测或权限链路的在线证据。

active Runtime 当前只执行 `conversation.list/read`。Artifact、消息发送、外部 MCP 和其他写 Capability 继续保持关闭。

## 2. 前置证据

执行前必须归档并复核以下内容：

1. 同一 candidate 的五类 Eval 报告及 canonical Suite SHA-256，结论满足 promotion policy。
2. `user_gray` release manifest，candidate 与运行镜像版本一致。
3. 共享 Kafka、Temporal、Core Capability RPC 和模型 Provider 的连通性与只读 smoke 证据。
4. Operator review/grant、观察窗口负责人和可执行回滚工单。
5. 活动会话与现有任务影响评估。没有明确维护窗口或批准时，不启动共享 Compose project。

离线 fixture、隔离 Compose smoke 和静态渲染只覆盖本地契约，不能替代以上证据。

## 3. 受控输入

`deploy/microservices/agent-active.yml` 要求以下输入，缺少任一项时 Compose 渲染失败：

| 输入 | 用途 |
| --- | --- |
| `DIPOLE_AGENT_RELEASE_MANIFEST_FILE` | 只读挂载的 `user_gray` manifest 文件 |
| `DIPOLE_AGENT_CANDIDATE_VERSION` | 与 manifest 和镜像一致的候选版本 |
| `DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID` | 独立的 active consumer group |
| `DIPOLE_AGENT_MODEL_PROVIDER_NAME` | OpenAI-compatible Provider 的 route 前缀 |
| `DIPOLE_AGENT_MODEL_BASE_URL` | HTTPS Provider endpoint；loopback HTTP 仅限开发 |
| `DIPOLE_AGENT_MODEL_API_KEY` | 从部署 Secret 注入，禁止写入 `.env`、命令历史或证据正文 |
| `DIPOLE_AGENT_MODEL_ROUTES` | 与 Provider name 前缀一致的有序模型 route |
| `DIPOLE_AGENT_MODEL_CONTEXT_PROFILES` | v2 Context Compiler 的严格 JSON profile |
| `DIPOLE_AGENT_TEMPORAL_ADDRESS` | 共享 Temporal endpoint |
| `DIPOLE_AGENT_TEMPORAL_NAMESPACE` | 目标 namespace |
| `DIPOLE_AGENT_TEMPORAL_TASK_QUEUE` | 独立 active task queue |

overlay 固定 `DIPOLE_AGENT_MODEL_MODE=ai_sdk`、`DIPOLE_AGENT_CONTEXT_COMPILER_VERSION=v2`、`DIPOLE_AGENT_TEMPORAL_ENABLED=true` 和 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=read_active`。

同一 overlay 固定 `direct_target`、Memory、Control、MCP Server 和 External MCP 为关闭。host 环境即使带有这些基础 Compose 开关，也不能在 user-gray read profile 中扩张 Capability 边界。

## 4. 渲染与启动

在隔离 project 目录中准备 Secret 注入后，先进行无副作用渲染：

```bash
docker compose \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-active.yml \
  config --quiet
```

复核渲染输出中的 Runtime mode、独立 consumer group、Temporal task queue 和只读 manifest 挂载。不要将含 API key 的完整 `config --format json` 输出写入日志或工单。

获得维护窗口批准后，使用独立 `COMPOSE_PROJECT_NAME` 启动并等待 readiness。启动后检查 `/livez`、`/readyz`、Temporal Worker 状态、Kafka consumer lag、Core RPC mTLS 和模型审计记录。每项检查都要记录 revision、candidate、manifest SHA-256、时间窗口和操作者，不记录 prompt、消息正文、API key 或 Tool 参数。

## 5. 回滚

出现 Provider、Temporal、Kafka、RPC、authority 或评测漂移时：

1. 停止 active project 的新调度并保留低敏诊断证据。
2. 移除 `agent-active.yml` override，恢复基础 Compose 的 Shadow Runtime。
3. 确认 active consumer group 停止、Temporal task queue 已排空或按工单暂停，并检查没有 active write Capability。
4. 撤销或过期对应 promotion grant，保留 manifest 与审计 Artifact 用于复盘。

禁止通过修改 release manifest 内容绕过阶段校验。下一次尝试应使用新的、重新评审的 manifest。

## 6. 关联资料

- [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)
- [Agent 前置能力清单](ai-readiness-checklist.md)
- [Agent OpenTelemetry 运维](agent-otel-operations.md)
- [架构债务台账](../architecture/ARCHITECTURE-DEBT.md)
- [微服务 Compose 说明](../../deploy/compose/README.md)
