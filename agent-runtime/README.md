# Dipole Agent Runtime

TypeScript Agent 执行面。当前 G2 foundation 固定可信 `ExecutionContext`、Capability Registry、resource-scope Policy Engine、Go 兼容 Task ID、Kafka shadow consumer 和 Fastify 健康面。

```bash
npm ci
npm test
npm run typecheck
npm run build
npm run generate:sql
```

运行独立 Kafka shadow consumer：

```bash
DIPOLE_AGENT_KAFKA_ENABLED=true \
DIPOLE_AGENT_KAFKA_BROKERS=127.0.0.1:9092 \
DIPOLE_AGENT_KAFKA_GROUP_ID=dipole-agent-shadow-v1 \
DIPOLE_AGENT_KAFKA_TOPIC_PREFIX=dipole \
DIPOLE_AGENT_KAFKA_FAILURE_MAX_ATTEMPTS=3 \
DIPOLE_AGENT_LEDGER_MODE=mysql \
DIPOLE_AGENT_MODEL_MODE=metadata \
DIPOLE_AGENT_MYSQL_HOST=127.0.0.1 \
DIPOLE_AGENT_MYSQL_USER=dipole_agent \
DIPOLE_AGENT_MYSQL_PASSWORD=change-me \
DIPOLE_AGENT_MYSQL_DATABASE=dipole \
npm start
```

Runtime 只接受 `message.direct.created` 的兼容 v1 envelope，使用独立 `dipole-agent-shadow-*` consumer group，并在 consumer 启动完成后开放 `/readyz`。默认物理 topic 为 `dipole.message.direct.created`，启动时创建并校验 main、`.retry`、`.dead` 的分区与副本配置。冷启动时 topic metadata 尚未收敛会执行有界重连，每次失败均断开旧 consumer。

## Temporal G3 foundation

Temporal Worker 默认关闭，默认注册无副作用的 foundation Activity，不接管 Kafka Shadow 流量：

```bash
DIPOLE_AGENT_TEMPORAL_ENABLED=true \
DIPOLE_AGENT_TEMPORAL_ADDRESS=127.0.0.1:7233 \
DIPOLE_AGENT_TEMPORAL_NAMESPACE=default \
DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-task-v1 \
DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=foundation \
npm start
```

Workflow ID 固定为 `dipole-agent-task/{task_id}`。运行中重复启动复用现有 Workflow，终态 Task 拒绝重复启动。模型调用、Capability RPC、持久化和副作用重试均须通过 Activity；foundation 的 Step Activity 只返回受控失败，用于验证 Worker 部署、恢复和运维边界。

显式设置 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=persistent_shadow` 后，Worker 使用既有 Agent Capability RPC 执行 Task/Run admission，并在 Workflow 终止前精确提交 completed、failed 或 cancelled Run。`wait_approval` 会先持久化 capability/scope/arguments/nonce 绑定；只有 request/approval ID 匹配且 Core 确认 actor 为 Task principal 的 Signal 才能完成 approved/revoked 并恢复 Workflow。该模式要求 `DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true` 及对应 target、共享密钥或 mTLS 配置。Workflow starter 和未来 Signal bridge 必须来自可信认证入口，模型无权设置 principal。当前 Kafka consumer 不启动 Workflow，模型、Capability Step 和权威 Task 状态继续由既有路径持有。

显式设置 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=read_shadow` 后，Kafka consumer 只负责 EventLedger claim 和稳定 Workflow 启动，ContextCompiler、ModelRouter、Plan/Step 持久化与 `conversation.list` 在 Temporal Activity 中执行。该模式同时要求 migration v23、`LEDGER_MODE=mysql`、`MODEL_MODE=ai_sdk`、模型 routes、Capability RPC 和 Temporal；Task、Run、admission 与原始事件必须精确绑定。成功模型输出写入 `agent_model_calls.output_json`，Activity 重试先恢复并重新校验该输出，已完成 Step 也不会再次执行。回滚时恢复 `persistent_shadow` 或 `foundation`，Compose 默认仍为 Temporal disabled + `foundation`。

Agent 镜像使用 Node 22 Bookworm slim。Temporal Native Core 发布为 GNU libc 二进制，Alpine/musl 镜像无法启用 Worker。

真实 Temporal dev server 契约默认不进入快速测试，可显式运行：

```bash
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm test -- --run src/temporal/agent-task-workflow.integration.test.ts
```

Worker 与 Core RPC 在线时可执行只读 Workflow projection 对账：

```bash
DIPOLE_AGENT_TEMPORAL_ENABLED=true \
DIPOLE_AGENT_TEMPORAL_ADDRESS=127.0.0.1:7233 \
DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true \
DIPOLE_AGENT_CAPABILITY_RPC_TARGET=127.0.0.1:9090 \
DIPOLE_INTERNAL_RPC_SHARED_SECRET=change-me \
npm run reconcile:projection -- --page-size=100 --max-examples=100
```

命令输出 `dipole.agent.projection-reconcile.v1` JSON。全部 Task 为 `match` 时退出 0，发现 missing/stale/ahead/conflict/unavailable 时退出 2。它通过 Core 私有 RPC 分页读取固定 shadow cohort，只读 Temporal Query/Describe，不修改 Task、Run 或 Workflow。

将同一候选版本的对账观察和 Eval 结果整理为 `dipole.agent.shadow-promotion-evidence.v1` 后，可执行：

```bash
npm run promotion:check -- --evidence=/path/to/evidence.json
```

策略要求连续 24 小时、至少 24 个观察点、最大间隔 90 分钟、累计至少 100 个 Task、零 projection 异常与 unavailable，并要求 projection/outcome/trajectory/permission Eval 全通过。eligible 只用于人工评审，命令不修改配置或运行时权威。

确认 Temporal 证据后，操作员可生成短时效修复候选 Artifact：

```bash
npm run repair:propose -- --input=/path/to/repair-input.json
```

提案绑定 Task、操作员声明、工单、原因、投影/Temporal 证据和一小时有效期，并生成稳定 SHA-256。当前没有 apply 命令，提案也尚未经过服务端签名和持久审批，不能作为已授权修复。

Shadow 模式仅生成并审计 plan，Policy Engine 拒绝 write/destructive capability。微服务默认使用 MySQL EventLedger，通过 Event ID/Task ID 唯一约束、claim token 与 lease 收敛重启和多副本重复投递；`memory` 只用于显式本地回滚。无效事件直接进入 dead，瞬时处理错误按 `retry_attempt` 有界重试；转移发布失败会让 handler 拒绝完成。migration v20 将 Plan 保存为不可变 Task 快照，并按顺序保存处于 `planned` 状态的结构化 capability Step；远程只读执行与 Step 终态将在 Agent Capability RPC 接入后启用。

模型调用默认关闭。显式开启 AI SDK shadow planner 时配置有序 route 与预算，并通过 `AI_GATEWAY_API_KEY` 提供 Gateway 凭据：

```bash
DIPOLE_AGENT_MODEL_MODE=ai_sdk \
DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini,anthropic/claude-sonnet-4.5 \
DIPOLE_AGENT_MODEL_MAX_CALLS=2 \
DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS=15000 \
DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS=512 \
AI_GATEWAY_API_KEY=... \
npm start
```

Runtime 按 route 顺序降级，失败调用同样消耗 `MAX_CALLS`；AI SDK 内部 retry 固定为 0。模型输出经过 Zod 校验，只能规划显式允许的只读 capability，并输出有序 `steps[]`。`ai_sdk` 模式强制使用 MySQL：ModelRouter 在每次 provider 调用前通过 ModelAuditStore 预留 Task slot，持久化 route、attempt、input/output Token、结构化输出、latency 与终态；Kafka 或 Temporal 重投不能刷新预算。

微服务环境使用根目录 `docker-compose.microservices.yml` 的 `agent` 服务；容器固定 Node 22，默认仅启用 Kafka 与 Agent 自有 MySQL ledger。Capability RPC 和 Temporal 均需通过显式开关及凭据启用。
