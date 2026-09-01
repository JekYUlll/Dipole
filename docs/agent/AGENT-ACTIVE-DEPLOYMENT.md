# Agent Active 部署运行手册

本文档约束 `read_active` 的 user-gray 部署。基础 Compose 固定为 Shadow；只有显式加载 active overlay 才会请求 active 依赖。

## 1. 边界

`docker compose config` 通过证明部署输入完整。它不提供 Kafka、Temporal、Capability RPC、模型 Provider、评测或权限链路的在线证据。

active Runtime 默认只执行 `conversation.list/read`。`DIPOLE_AGENT_INTERACTIVE_MESSAGE_WRITE_ENABLED=true` 是独立的候选开关：它只允许 owner 在直属 Agent 会话发出显式 `/send <内容>`，Task 先进入 `waiting_approval`，approved Signal 后通过既有 grant、一次性 consume、Tool Invocation 与 Core 消息命令链路执行一条 `system_message`。当前 active overlay 不设置该开关，因此 Artifact、消息发送、外部 MCP 和其他写 Capability 继续保持关闭。

## 2. 前置证据

执行前必须归档并复核以下内容：

1. 同一 candidate 的五类 Eval 报告及 canonical Suite SHA-256，结论满足 promotion policy。
2. `user_gray` release manifest，candidate 与运行镜像版本一致。
3. 共享 Kafka、Temporal、Core Capability RPC 和模型 Provider 的连通性与只读 smoke 证据。
4. Operator review/grant、观察窗口负责人和可执行回滚工单。
5. 活动会话与现有任务影响评估。没有明确维护窗口或批准时，不启动共享 Compose project。

离线 fixture、隔离 Compose smoke 和静态渲染只覆盖本地契约，不能替代以上证据。

### 开发期质量基线

2026-08-31，Remote GPU 候选工作树在 `8e99bde7`、Node `22.12.0` 上完成 Agent Runtime 完整门禁：`134 passed / 9 skipped` 测试文件、`703 passed / 30 skipped` 测试，以及 `typecheck` 和 production `build`。验证后工作树干净，过程中未启动 Compose、Kafka、Temporal 或 active authority。该结果可用于确认候选的 TypeScript 质量基线；共享环境的 Kafka trigger、Gateway-to-Core revoke、overlay 回滚和 24 小时观察仍需按本手册完成。

## 3. 受控输入

`deploy/microservices/agent-active.yml` 要求以下输入，缺少任一项时 Compose 渲染失败：

| 输入 | 用途 |
| --- | --- |
| `DIPOLE_AGENT_RELEASE_MANIFEST_FILE` | 只读挂载的 `user_gray` manifest 文件 |
| `DIPOLE_AGENT_CANDIDATE_VERSION` | 与 manifest 和镜像一致的候选版本 |
| `DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID` | 独立的 active consumer group，必须以 `dipole-agent-active-` 开头 |
| `DIPOLE_AGENT_MODEL_PROVIDER_NAME` | OpenAI-compatible Provider 的 route 前缀 |
| `DIPOLE_AGENT_MODEL_BASE_URL` | HTTPS Provider endpoint；loopback HTTP 仅限开发 |
| `DIPOLE_AGENT_MODEL_API_KEY` | 从部署 Secret 注入，禁止写入 `.env`、命令历史或证据正文 |
| `DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS` | 仅在 Provider 已验证支持 JSON Schema 时设为 `true`，默认 `false` |
| `DIPOLE_AGENT_MODEL_OUTPUT_MODE` | `json_schema`（默认）或经验证的单次调用 `json_text` fallback |
| `DIPOLE_AGENT_MODEL_ROUTES` | 与 Provider name 前缀一致的有序模型 route |
| `DIPOLE_AGENT_MODEL_CONTEXT_PROFILES` | v2 Context Compiler 的严格 JSON profile |
| `DIPOLE_AGENT_TEMPORAL_ADDRESS` | 共享 Temporal endpoint |
| `DIPOLE_AGENT_TEMPORAL_NAMESPACE` | 目标 namespace |
| `DIPOLE_AGENT_TEMPORAL_TASK_QUEUE` | 独立 active task queue |

overlay 固定 `DIPOLE_AGENT_MODEL_MODE=ai_sdk`、`DIPOLE_AGENT_CONTEXT_COMPILER_VERSION=v2`、`DIPOLE_AGENT_TEMPORAL_ENABLED=true` 和 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=read_active`。

同一 overlay 固定 `direct_target`、Memory、retrieval、retrieval-to-Context、Control、MCP Server 和 External MCP 为关闭。host 环境即使带有这些基础 Compose 开关，也不能在 user-gray read profile 中扩张 Capability 边界。

Runtime 也会在启动前执行相同的 active read profile 校验，因此直接使用环境变量启动时，开启上述任一入口都会 fail closed。

交互消息候选在共享环境启用前，还需要同一 revision 的 Core/Temporal/Compose 真实 receipt：owner approve、deny、重复 consume、Activity 重试及回滚均必须记录消息副作用计数。隔离 Temporal 已验证提交后 `UNAVAILABLE` / `DEADLINE_EXCEEDED` 的稳定命令标识与重试收敛，见 [Interactive Active Retry Receipt](AGENT-INTERACTIVE-ACTIVE-RETRY-RECEIPT.md)；受认证的 Core-to-Message gRPC 回包丢失恢复见 [Interactive Message Transport Receipt](AGENT-INTERACTIVE-ACTIVE-MESSAGE-TRANSPORT-RECEIPT.md)，SQLC MySQL 持久化 smoke 见 [Interactive Message MySQL Receipt](AGENT-INTERACTIVE-ACTIVE-MESSAGE-MYSQL-RECEIPT.md)。这些证据都不能替代真实 Compose、部分副作用回滚或共享环境 receipt。

`deploy/microservices/agent-interactive-active.yml` 是 `agent-active.yml` 之上的独立 overlay。它将 Activity 切换到 `interactive_active`，开启 Agent Control API、Gateway 的任务控制转发与 `/send` 执行器，并要求 `DIPOLE_AGENT_INTERACTIVE_TASK_QUEUE` 使用 `dipole-agent-interactive-` 前缀及独立的 `DIPOLE_AGENT_CONTROL_SECRET`。Artifact 与 MCP 入口在该 overlay 中保持关闭。只有归档本节要求的共享环境 receipt 后才允许加载该 overlay。

### 隔离 Compose 验收

开发环境可执行以下 smoke，将同一受控 profile 放入一次性的 Compose
project。脚本生成临时 mTLS 证书、release manifest、active Kafka group、
Temporal queue 与 promotion grant；Gateway 只绑定 loopback，结束时会撤销
grant 并删除该 project 的 volumes。`/send` 场景不调用模型，因此脚本使用
无效的本地占位 Provider endpoint，避免将开发凭据发送到外部网络。

```bash
BUILD_IMAGE=1 scripts/smoke-agent-interactive-active-compose.sh
```

验收分为两条确定性路径：并发 `denied` 重放必须收敛为零 Tool/Message
副作用；并发 `approved` 重放必须收敛为一次 approval consume、一次完成的
Tool Invocation、一个稳定 client message ID、一条 Message 和两条收件人
Sync Timeline 项。Message command 通过 Kafka 持久化时，Core 只会对临时
`absent` receipt 在 `2s` 内确认，避免在已提交消息尚未投影前固化冲突；读取
错误、nil receipt 和超时后的 `absent` 均保持失败关闭。`KEEP_STACK=1` 仅用于
隔离排障，保留 stack 后仍必须确认 promotion grant 已撤销。该 smoke 覆盖干净
Compose 的审批重放与异步 receipt 确认，不覆盖 Core/Message 响应丢失、Worker
替换、部分副作用 rollback、浏览器 HITL、共享 tenant 或容量结论；这些继续由
`AD-009` 管理。

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

提交共享环境前，可先运行隔离的跨语言 mTLS RPC drill：

隔离验证按以下顺序执行，四个步骤分别覆盖不同边界，均不代表 active 默认路径已启用：

1. `DIPOLE_GO_BIN=/home/admin1/.local/go-1.27.0/bin/go GOTOOLCHAIN=local scripts/test-agent-memory-promotion-mysql-contract.sh` 验证 migration、持久 Task/Run、grant、candidate/review、幂等晋级、撤销拒绝，以及临时 CA 下的 Core receipt adapter loopback TCP+mTLS；脚本创建并清理独立 MySQL 容器。
2. `DIPOLE_GO_BIN=/home/admin1/.local/go-1.27.0/bin/go GOTOOLCHAIN=local scripts/drill-agent-memory-promotion-rpc.sh` 验证 TypeScript generated client 到 Go fixture 的 mTLS 身份、protobuf 和低敏回包绑定。
3. 运行现有 Temporal receipt retry integration，确认同一 prepared receipt 的 Activity 重试语义；该测试仍使用 commit stub。
4. `DIPOLE_GO_BIN=/home/admin1/.local/go-1.27.0/bin/go DIPOLE_NODE_BIN=/home/admin1/.local/node-22.12.0/bin/node GOTOOLCHAIN=local scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh` 启动临时 MySQL、实际 Core receipt adapter 与 loopback mTLS fixture，并运行 Temporal Worker；它会在首个持久提交后故意失败一次，再验证同一 receipt 重试返回同一条 MySQL Memory，同时撤销一个已 admission Run 的 grant 并确认后续 receipt 被拒绝且零写入，最后由 owner application control 撤销首个已写入 Memory。

2026-08-30 已在 Remote GPU 的一次性 worktree 上以 Node 22 执行该步骤：内存 Temporal test server 的两个 integration case 通过，受控第一次 commit 失败后第二次提交仍复用同一 receipt SHA-256。该记录只证明 Temporal workflow 与 stub 的 durable retry 语义，不能作为 Core、MySQL grant、Kafka 或 active overlay 的联合验收。

2026-08-30 已在 Remote GPU 一次性 worktree 上通过第 4 步，证明同 receipt 的跨进程 durable retry、MySQL 幂等、admission 后 grant 撤销拒绝，以及 owner application control 的持久 Memory revoke。Gateway 的认证 HTTP 到 mTLS Core RPC revoke 链路已由单元/contract 测试覆盖；该演练使用临时 CA、临时数据库与内存 Temporal，不连接 Kafka，也未形成共享 Gateway-to-Core 运行记录或 overlay 回滚证据；这些仍是共享环境接管的必要条件。

开发期可使用以下显式门禁复核纯内存 Temporal 的 Agent Task 语义：

```bash
cd services/agent-runtime
npm run test:temporal:integration
```

它覆盖幂等启动、Worker replacement、审批、Elicitation 输入/超时、取消、步数预算、后效重放和 reviewed Memory receipt 重试。该命令不启动 Compose、Kafka、Core、MySQL 或 active authority，不能替代本章的共享环境前置证据。

```bash
DIPOLE_GO_BIN=/path/to/go-1.26-or-newer/bin/go \
  scripts/drill-agent-memory-promotion-rpc.sh
```

该脚本使用临时 CA、loopback Go fixture 与 TypeScript generated gRPC client，验证 `dipole-agent` 身份、prepared receipt 编码和低敏 response binding。它不启动 Docker、Temporal、Kafka 或 MySQL，不读取 candidate/review 持久状态，也不会写入任何真实 Memory；结果只能作为 RPC 传输和 client/server 契约证据。

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
