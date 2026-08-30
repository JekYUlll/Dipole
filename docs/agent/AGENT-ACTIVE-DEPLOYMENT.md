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

Runtime 也会在启动前执行相同的 active read profile 校验，因此直接使用环境变量启动时，开启上述任一入口都会 fail closed。

## 4. Reviewed Memory 提交扩展

`deploy/microservices/agent-memory-promotion.yml` 是 `agent-active.yml` 之上的独立 overlay，默认不加载。它只允许为已审核的 receipt 增加 `promotion_active` Temporal Activity，同时打开 Core 的 receipt commit Adapter。该 overlay 不改变 candidate 生成、Memory 召回、消息发送、Control 或 MCP 的关闭状态。

除第 3 节的全部输入外，operator 还必须显式提供：

| 输入 | 用途 |
| --- | --- |
| `DIPOLE_AGENT_MEMORY_PROMOTION_AUTHORITY=operator_approved` | 将经过维护窗口审批的 authority 绑定到 Runtime 启动；缺失或其他值均 fail closed。 |

Runtime 启动会同时校验 active Runtime、`promotion_active`、Temporal、Capability RPC mTLS、operator authority 与只读 Capability surface；Core 在自身启动时仍独立要求 receipt commit 开关与 mTLS。Core application 会继续基于持久化 Task/Run、active admission 和有效 promotion grant 重新授权，运行时环境变量不提供写入授权。

受控渲染命令：

```bash
docker compose \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-active.yml \
  -f deploy/microservices/agent-memory-promotion.yml \
  config --quiet
```

执行前归档 grant、manifest SHA-256、Core/Runtime revision 和回滚工单；完成后至少演练一次有效 receipt 的 Activity 重试与一次失效 grant 拒绝。缺少共享环境证据时，该 overlay 继续只作为受控候选，不提升为默认路径。

演练结束后将脱敏结果写入独立 JSON，再执行：

```bash
cd services/agent-runtime
npm run promotion:memory-worker-drill -- --evidence=/secure/path/worker-drill.json
```

该 CLI 只接受同一候选的 revision、manifest/configuration/promotion-evidence 摘要、grant ID、Temporal queue、首个 commit、重试结果、失效 grant 拒绝和回滚结果。仅 `eligible` 表示记录完整；它不会访问上述系统或代替原始日志、监控快照和审批工单。

## 5. 渲染与启动

在隔离 project 目录中准备 Secret 注入后，先进行无副作用渲染：

```bash
docker compose \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-active.yml \
  config --quiet
```

复核渲染输出中的 Runtime mode、独立 consumer group、Temporal task queue 和只读 manifest 挂载。不要将含 API key 的完整 `config --format json` 输出写入日志或工单。

获得维护窗口批准后，使用独立 `COMPOSE_PROJECT_NAME` 启动并等待 readiness。启动后检查 `/livez`、`/readyz`、Temporal Worker 状态、Kafka consumer lag、Core RPC mTLS 和模型审计记录。每项检查都要记录 revision、candidate、manifest SHA-256、时间窗口和操作者，不记录 prompt、消息正文、API key 或 Tool 参数。

## 6. 回滚

出现 Provider、Temporal、Kafka、RPC、authority 或评测漂移时：

1. 停止 active project 的新调度并保留低敏诊断证据。
2. 移除 `agent-active.yml` override，恢复基础 Compose 的 Shadow Runtime。
3. 确认 active consumer group 停止、Temporal task queue 已排空或按工单暂停，并检查没有 active write Capability。
4. 撤销或过期对应 promotion grant，保留 manifest 与审计 Artifact 用于复盘。

禁止通过修改 release manifest 内容绕过阶段校验。下一次尝试应使用新的、重新评审的 manifest。

## 7. 关联资料

- [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)
- [Agent 前置能力清单](ai-readiness-checklist.md)
- [Agent OpenTelemetry 运维](agent-otel-operations.md)
- [架构债务台账](../architecture/ARCHITECTURE-DEBT.md)
- [微服务 Compose 说明](../../deploy/compose/README.md)
