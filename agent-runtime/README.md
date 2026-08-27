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

Temporal Worker 默认关闭，当前仅注册无副作用的 foundation Activity，不接管 Kafka Shadow 流量：

```bash
DIPOLE_AGENT_TEMPORAL_ENABLED=true \
DIPOLE_AGENT_TEMPORAL_ADDRESS=127.0.0.1:7233 \
DIPOLE_AGENT_TEMPORAL_NAMESPACE=default \
DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-task-v1 \
npm start
```

Workflow ID 固定为 `dipole-agent-task/{task_id}`。运行中重复启动复用现有 Workflow，终态 Task 拒绝重复启动。模型调用、Capability RPC、持久化和副作用重试均须通过 `executeAgentTaskStep` Activity；当前 foundation Activity 只返回受控失败，用于验证 Worker 部署、恢复和运维边界。

Agent 镜像使用 Node 22 Bookworm slim。Temporal Native Core 发布为 GNU libc 二进制，Alpine/musl 镜像无法启用 Worker。

真实 Temporal dev server 契约默认不进入快速测试，可显式运行：

```bash
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm test -- --run src/temporal/agent-task-workflow.integration.test.ts
```

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

Runtime 按 route 顺序降级，失败调用同样消耗 `MAX_CALLS`；AI SDK 内部 retry 固定为 0。模型输出经过 Zod 校验，只能规划显式允许的只读 capability，并输出有序 `steps[]`。`ai_sdk` 模式强制使用 MySQL：ModelRouter 在每次 provider 调用前通过 migration v19 ModelAuditStore 预留 Task slot，并持久化 route、attempt、input/output Token、latency 与终态；Kafka 重投不能刷新预算。

微服务环境使用根目录 `docker-compose.microservices.yml` 的 `agent` 服务；容器固定 Node 22，只连接 Kafka 与 Agent 自有 MySQL ledger，不连接 Redis 或 Go 内部 RPC。
